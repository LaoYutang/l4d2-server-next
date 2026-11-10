package logic

import (
	"context"
	"errors"
	"fmt"
	"l4d2-manager-next/db"
	"l4d2-manager-next/model"
	"l4d2-manager-next/utility"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

const (
	auditQueueCapacity = 2048
	auditBatchSize     = 100
	auditFlushInterval = 250 * time.Millisecond
	auditWarningWindow = time.Minute
)

var ErrAuditDatabaseUnavailable = errors.New("audit database is unavailable")

type AuditListFilter struct {
	Page      int
	PageSize  int
	StartTime int64
	EndTime   int64
	Role      string
	IP        string
	Path      string
	Success   *bool
	Keyword   string
}

type AuditLogItem struct {
	model.AuditLog
	Location string `json:"location"`
}

type auditWriter struct {
	database    *gorm.DB
	queue       chan model.AuditLog
	stopCh      chan struct{}
	doneCh      chan struct{}
	stopOnce    sync.Once
	dropped     atomic.Uint64
	lastWarning atomic.Int64
	cleanupCtx  context.Context
	stopCleanup context.CancelFunc
}

var (
	defaultAuditWriter   *auditWriter
	defaultAuditWriterMu sync.RWMutex
	auditUnavailableDrop atomic.Uint64
	auditUnavailableWarn atomic.Int64
)

func newAuditWriter(database *gorm.DB, capacity int) *auditWriter {
	ctx, cancel := context.WithCancel(context.Background())
	return &auditWriter{
		database:    database,
		queue:       make(chan model.AuditLog, capacity),
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
		cleanupCtx:  ctx,
		stopCleanup: cancel,
	}
}

// StartAuditWriter starts the process-lifetime audit writer and retention task.
// It intentionally has no shutdown drain because manager shutdown is immediate.
func StartAuditWriter() {
	if db.AuditDB == nil {
		log.Printf("[AUDIT] database unavailable; async audit writer was not started")
		return
	}

	writer := newAuditWriter(db.AuditDB, auditQueueCapacity)
	defaultAuditWriterMu.Lock()
	if defaultAuditWriter != nil {
		defaultAuditWriterMu.Unlock()
		writer.stopCleanup()
		return
	}
	defaultAuditWriter = writer
	defaultAuditWriterMu.Unlock()

	go writer.run()
}

// EnqueueAuditLog never waits for the database or for queue capacity.
func EnqueueAuditLog(entry model.AuditLog) {
	defaultAuditWriterMu.RLock()
	writer := defaultAuditWriter
	defaultAuditWriterMu.RUnlock()
	if writer == nil {
		warnAuditDrop("database unavailable", &auditUnavailableDrop, &auditUnavailableWarn)
		return
	}

	writer.enqueue(entry)
}

func (w *auditWriter) enqueue(entry model.AuditLog) bool {
	select {
	case w.queue <- entry:
		return true
	default:
		warnAuditDrop("queue full", &w.dropped, &w.lastWarning)
		return false
	}
}

func warnAuditDrop(reason string, counter *atomic.Uint64, lastWarning *atomic.Int64) {
	counter.Add(1)
	now := time.Now().UnixNano()
	last := lastWarning.Load()
	if last != 0 && time.Duration(now-last) < auditWarningWindow {
		return
	}
	if lastWarning.CompareAndSwap(last, now) {
		dropped := counter.Swap(0)
		log.Printf("[AUDIT] dropped %d database audit record(s): %s", dropped, reason)
	}
}

func (w *auditWriter) run() {
	ticker := time.NewTicker(auditFlushInterval)
	defer ticker.Stop()
	cleanupTicker := time.NewTicker(auditCleanupInterval)
	defer cleanupTicker.Stop()
	w.runWithTicks(time.Now, ticker.C, cleanupTicker.C)
}

func (w *auditWriter) runWithTicks(now func() time.Time, flushTicks, cleanupTicks <-chan time.Time) {
	defer close(w.doneCh)
	defer w.stopCleanup()

	// Cleanup shares the writer's goroutine, so it never competes with inserts
	// for SQLite's write lock or relies on a connection-local busy_timeout.
	cleanupStep := make(chan struct{}, 1)
	cleanupCutoff := now().Add(-auditRetention).Unix()
	cleanupStep <- struct{}{}

	batch := make([]model.AuditLog, 0, auditBatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		w.writeBatch(batch)
		batch = batch[:0]
	}

	for {
		// Give shutdown priority even when a large cleanup has more batches.
		select {
		case <-w.stopCh:
			flush()
			return
		default:
		}
		select {
		case entry := <-w.queue:
			batch = append(batch, entry)
			if len(batch) >= auditBatchSize {
				flush()
			}
		case <-flushTicks:
			flush()
		case <-cleanupTicks:
			// Use execution time rather than a possibly delayed tick timestamp.
			cleanupCutoff = now().Add(-auditRetention).Unix()
			select {
			case cleanupStep <- struct{}{}:
			default:
			}
		case <-cleanupStep:
			flush()
			ctx, cancel := context.WithTimeout(w.cleanupCtx, auditCleanupTimeout)
			deleted, err := deleteExpiredAuditBatch(ctx, w.database, cleanupCutoff)
			cancel()
			if err != nil {
				if w.cleanupCtx.Err() == nil {
					log.Printf("[AUDIT] failed to clean up expired records; will retry on the next daily check: %v", err)
				}
				continue
			}
			if deleted == auditCleanupBatchSize {
				// Return to the select between bounded deletes to keep accepting
				// and flushing queued operations during a large initial cleanup.
				cleanupStep <- struct{}{}
			}
		case <-w.stopCh:
			flush()
			return
		}
	}
}

func (w *auditWriter) stopForTest() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
		w.stopCleanup()
	})
	<-w.doneCh
}

func (w *auditWriter) writeBatch(batch []model.AuditLog) {
	if w.database == nil {
		log.Printf("[AUDIT] dropped %d database audit record(s): database unavailable", len(batch))
		return
	}

	delays := []time.Duration{100 * time.Millisecond, 500 * time.Millisecond}
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = w.database.CreateInBatches(batch, auditBatchSize).Error
		if err == nil {
			return
		}
		if attempt < len(delays) {
			time.Sleep(delays[attempt])
		}
	}
	log.Printf("[AUDIT] dropped %d database audit record(s) after retries: %v", len(batch), err)
}

func ListAuditLogs(filter AuditListFilter) ([]AuditLogItem, int64, error) {
	if db.AuditDB == nil {
		return nil, 0, ErrAuditDatabaseUnavailable
	}

	query := db.AuditDB.Model(&model.AuditLog{})
	if filter.StartTime > 0 {
		query = query.Where("time >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("time <= ?", filter.EndTime)
	}
	if filter.Role != "" {
		query = query.Where("role = ?", filter.Role)
	}
	if filter.IP != "" {
		query = query.Where("ip LIKE ?", containsPattern(filter.IP))
	}
	if filter.Path != "" {
		query = query.Where("path LIKE ?", containsPattern(filter.Path))
	}
	if filter.Success != nil {
		query = query.Where("success = ?", *filter.Success)
	}
	if filter.Keyword != "" {
		query = query.Where("detail LIKE ?", containsPattern(filter.Keyword))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	var records []model.AuditLog
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("time DESC").Order("id DESC").Offset(offset).Limit(filter.PageSize).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	return buildAuditLogItems(records, utility.GetLocation), total, nil
}

func buildAuditLogItems(records []model.AuditLog, lookupLocation func(string) string) []AuditLogItem {
	items := make([]AuditLogItem, len(records))
	locations := make(map[string]string, len(records))
	for index, record := range records {
		location, found := locations[record.IP]
		if !found {
			location = lookupLocation(record.IP)
			if location == "" {
				location = "未知"
			}
			locations[record.IP] = location
		}
		items[index] = AuditLogItem{
			AuditLog: record,
			Location: location,
		}
	}
	return items
}

func containsPattern(value string) string {
	return "%" + value + "%"
}
