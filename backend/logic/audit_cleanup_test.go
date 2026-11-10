package logic

import (
	"context"
	"errors"
	"l4d2-manager-next/model"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	_ "time/tzdata"

	"gorm.io/gorm"
)

type auditCleanupResult struct {
	deleted int64
	err     error
}

func observeAuditCleanup(t *testing.T, database *gorm.DB) <-chan auditCleanupResult {
	t.Helper()
	results := make(chan auditCleanupResult, 64)
	if err := database.Callback().Delete().After("gorm:commit_or_rollback_transaction").
		Register("audit:test_observe_cleanup", func(tx *gorm.DB) {
			results <- auditCleanupResult{tx.RowsAffected, tx.Error}
		}); err != nil {
		t.Fatal(err)
	}
	return results
}

func waitForAuditCleanup(t *testing.T, results <-chan auditCleanupResult) auditCleanupResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for audit cleanup")
		return auditCleanupResult{}
	}
}

func TestAuditCleanupRollingBoundaryAndTimeZones(t *testing.T) {
	for _, instant := range []string{"2026-03-20T12:34:56.789Z", "2026-11-10T12:34:56.789Z"} {
		for _, zone := range []string{"UTC", "Asia/Shanghai", "America/New_York"} {
			t.Run(instant+"/"+zone, func(t *testing.T) {
				location, err := time.LoadLocation(zone)
				if err != nil {
					t.Fatal(err)
				}
				now, err := time.Parse(time.RFC3339Nano, instant)
				if err != nil {
					t.Fatal(err)
				}
				now = now.In(location)
				cutoff := now.Unix() - 30*24*60*60
				database := openAuditTestDB(t)
				records := []model.AuditLog{
					{Time: cutoff - 1}, {Time: cutoff}, {Time: cutoff + 1},
					{Time: now.Unix()}, {Time: now.Unix() + 3600},
				}
				if err := database.Create(&records).Error; err != nil {
					t.Fatal(err)
				}
				results := observeAuditCleanup(t, database)
				writer := newAuditWriter(database, auditQueueCapacity)
				go writer.runWithTicks(func() time.Time { return now }, nil, nil)
				defer writer.stopForTest()
				result := waitForAuditCleanup(t, results)
				if result.err != nil || result.deleted != 1 {
					t.Fatalf("cleanup result = %+v, want exactly one expired record", result)
				}
				var remaining []model.AuditLog
				if err := database.Order("time").Find(&remaining).Error; err != nil {
					t.Fatal(err)
				}
				if len(remaining) != 4 || remaining[0].Time != cutoff {
					t.Fatalf("boundary or newer records lost: %+v", remaining)
				}
			})
		}
	}
}

func TestAuditCleanupBoundedBatchesOnlyDeleteAuditRecords(t *testing.T) {
	database := openAuditTestDB(t)
	if err := database.AutoMigrate(&model.SystemMetric{}); err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&model.SystemMetric{Timestamp: 1}).Error; err != nil {
		t.Fatal(err)
	}
	records := make([]model.AuditLog, auditCleanupBatchSize+3)
	for i := range records {
		records[i].Time = 1
	}
	if err := database.CreateInBatches(records, auditBatchSize).Error; err != nil {
		t.Fatal(err)
	}
	for _, expected := range []int64{auditCleanupBatchSize, 3, 0} {
		deleted, err := deleteExpiredAuditBatch(context.Background(), database, 2)
		if err != nil || deleted != expected {
			t.Fatalf("deleted = %d, err = %v; want %d", deleted, err, expected)
		}
	}
	var metrics int64
	if err := database.Model(&model.SystemMetric{}).Count(&metrics).Error; err != nil || metrics != 1 {
		t.Fatalf("cleanup affected another table: metrics=%d err=%v", metrics, err)
	}
}

func TestAuditCleanupRunsOnStartupAndDailyUsingExecutionTime(t *testing.T) {
	if auditCleanupInterval != 24*time.Hour {
		t.Fatalf("daily interval = %v", auditCleanupInterval)
	}
	database := openAuditTestDB(t)
	start := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	cutoff := start.Unix() - 30*24*60*60
	records := []model.AuditLog{{Time: cutoff - 1}, {Time: cutoff + 1}}
	if err := database.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	results := observeAuditCleanup(t, database)
	var current atomic.Int64
	current.Store(start.Unix())
	ticks := make(chan time.Time, 1)
	writer := newAuditWriter(database, auditQueueCapacity)
	go writer.runWithTicks(func() time.Time { return time.Unix(current.Load(), 0) }, nil, ticks)
	defer writer.stopForTest()
	if result := waitForAuditCleanup(t, results); result.err != nil || result.deleted != 1 {
		t.Fatalf("startup cleanup = %+v", result)
	}
	select {
	case result := <-results:
		t.Fatalf("unexpected cleanup without a daily tick: %+v", result)
	default:
	}
	current.Store(start.Add(24 * time.Hour).Unix())
	// A delayed ticker value must not cause cleanup to use a stale cutoff.
	ticks <- start
	if result := waitForAuditCleanup(t, results); result.err != nil || result.deleted != 1 {
		t.Fatalf("daily cleanup = %+v", result)
	}
	writer.stopForTest()
	ticks <- start.Add(48 * time.Hour)
	select {
	case result := <-results:
		t.Fatalf("cleanup continued after stop: %+v", result)
	default:
	}
}

func TestAuditCleanupSerializesWithConcurrentEnqueues(t *testing.T) {
	database := openAuditTestDB(t)
	if err := database.Exec("PRAGMA journal_mode = WAL;").Error; err != nil {
		t.Fatal(err)
	}
	// No busy_timeout is set: the worker must serialize its own SQLite writes.
	var active, overlaps atomic.Int64
	enter := func(*gorm.DB) {
		if active.Add(1) != 1 {
			overlaps.Add(1)
		}
	}
	leave := func(*gorm.DB) { active.Add(-1) }
	old := make([]model.AuditLog, 2*auditCleanupBatchSize+1)
	for i := range old {
		old[i].Time = 1
	}
	if err := database.CreateInBatches(old, auditBatchSize).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Callback().Create().Before("gorm:begin_transaction").Register("audit:test_enter_write", enter); err != nil {
		t.Fatal(err)
	}
	if err := database.Callback().Create().After("gorm:commit_or_rollback_transaction").Register("audit:test_leave_write", leave); err != nil {
		t.Fatal(err)
	}
	if err := database.Callback().Delete().Before("gorm:begin_transaction").Register("audit:test_enter_delete", enter); err != nil {
		t.Fatal(err)
	}
	if err := database.Callback().Delete().After("gorm:commit_or_rollback_transaction").Register("audit:test_leave_delete", leave); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	flushTicks, cleanupTicks := make(chan time.Time, 1), make(chan time.Time, 4)
	writer := newAuditWriter(database, auditQueueCapacity)
	go writer.runWithTicks(func() time.Time { return now }, flushTicks, cleanupTicks)
	defer writer.stopForTest()
	var producers sync.WaitGroup
	var dropped atomic.Int64
	for i := 0; i < 4; i++ {
		producers.Go(func() {
			for j := 0; j < auditBatchSize; j++ {
				if !writer.enqueue(model.AuditLog{Time: now.Unix(), Detail: "concurrent"}) {
					dropped.Add(1)
				}
			}
		})
		cleanupTicks <- now
	}
	producers.Wait()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var expired, fresh int64
		if err := database.Model(&model.AuditLog{}).Where("time = 1").Count(&expired).Error; err != nil {
			t.Fatal(err)
		}
		if err := database.Model(&model.AuditLog{}).Where("detail = ?", "concurrent").Count(&fresh).Error; err != nil {
			t.Fatal(err)
		}
		if expired == 0 && fresh == 4*auditBatchSize {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expired=%d fresh=%d, want 0 and %d", expired, fresh, 4*auditBatchSize)
		}
		select {
		case flushTicks <- now:
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	writer.stopForTest()
	if overlaps.Load() != 0 || active.Load() != 0 || dropped.Load() != 0 {
		t.Fatalf("overlapping writes=%d active=%d dropped=%d", overlaps.Load(), active.Load(), dropped.Load())
	}
}

func TestAuditCleanupFailureKeepsWriterAliveAndRetriesNextDay(t *testing.T) {
	database := openAuditTestDB(t)
	if err := database.Create(&model.AuditLog{Time: 1}).Error; err != nil {
		t.Fatal(err)
	}
	var fail atomic.Bool
	fail.Store(true)
	if err := database.Callback().Delete().Before("gorm:delete").Register("audit:test_fail_delete", func(tx *gorm.DB) {
		if fail.Load() {
			tx.AddError(errors.New("forced cleanup failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	results := observeAuditCleanup(t, database)
	now := time.Now()
	flushTicks, cleanupTicks := make(chan time.Time, 1), make(chan time.Time, 1)
	writer := newAuditWriter(database, auditQueueCapacity)
	go writer.runWithTicks(func() time.Time { return now }, flushTicks, cleanupTicks)
	defer writer.stopForTest()
	if result := waitForAuditCleanup(t, results); result.err == nil {
		t.Fatal("expected the startup database error")
	}
	for i := 0; i < auditBatchSize; i++ {
		writer.enqueue(model.AuditLog{Time: now.Unix(), Detail: "after cleanup failure"})
	}
	waitForAuditCount(t, database, auditBatchSize+1)
	select {
	case result := <-results:
		t.Fatalf("cleanup retried before the next daily tick: %+v", result)
	default:
	}
	fail.Store(false)
	cleanupTicks <- now.Add(24 * time.Hour)
	if result := waitForAuditCleanup(t, results); result.err != nil || result.deleted != 1 {
		t.Fatalf("recovery cleanup = %+v", result)
	}
	var records []model.AuditLog
	if err := database.Find(&records).Error; err != nil || len(records) != auditBatchSize || records[0].Detail != "after cleanup failure" {
		t.Fatalf("fresh audit write lost after cleanup failure: records=%+v err=%v", records, err)
	}
}

func TestAuditCleanupStopCancelsInFlightDelete(t *testing.T) {
	database := openAuditTestDB(t)
	entered := make(chan struct{})
	if err := database.Callback().Delete().Before("gorm:delete").Register("audit:test_wait_for_cancel", func(tx *gorm.DB) {
		close(entered)
		<-tx.Statement.Context.Done()
		tx.AddError(tx.Statement.Context.Err())
	}); err != nil {
		t.Fatal(err)
	}
	writer := newAuditWriter(database, auditQueueCapacity)
	go writer.run()
	defer writer.stopForTest()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("cleanup did not start")
	}
	stopped := make(chan struct{})
	go func() { writer.stopForTest(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("stop did not cancel the in-flight cleanup")
	}
}

func TestAuditCleanupUnavailableClosedAndCanceledDatabases(t *testing.T) {
	if _, err := deleteExpiredAuditBatch(context.Background(), nil, 2); !errors.Is(err, ErrAuditDatabaseUnavailable) {
		t.Fatalf("nil database error = %v", err)
	}
	database := openAuditTestDB(t)
	if err := database.Create(&model.AuditLog{Time: 1}).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := deleteExpiredAuditBatch(ctx, database, 2); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled cleanup error = %v", err)
	}
	var count int64
	if err := database.Model(&model.AuditLog{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("canceled cleanup changed records: count=%d err=%v", count, err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := deleteExpiredAuditBatch(context.Background(), database, 2); err == nil {
		t.Fatal("expected an error from the closed database")
	}
}
