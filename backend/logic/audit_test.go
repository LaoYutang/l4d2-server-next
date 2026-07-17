package logic

import (
	"errors"
	"l4d2-manager-next/db"
	"l4d2-manager-next/model"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "audit.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open audit database: %v", err)
	}
	if err := database.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate audit database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get audit sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return database
}

func waitForAuditCount(t *testing.T, database *gorm.DB, expected int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var count int64
		if err := database.Model(&model.AuditLog{}).Count(&count).Error; err == nil && count >= expected {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d audit records", expected)
}

func TestAuditWriterEnqueueDoesNotBlockWhenFull(t *testing.T) {
	writer := newAuditWriter(nil, 1)
	writer.lastWarning.Store(time.Now().UnixNano())

	if !writer.enqueue(model.AuditLog{Detail: "first"}) {
		t.Fatal("first audit record should fit in queue")
	}

	start := time.Now()
	if writer.enqueue(model.AuditLog{Detail: "second"}) {
		t.Fatal("second audit record should be dropped when queue is full")
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("full queue enqueue blocked for %v", elapsed)
	}
	if dropped := writer.dropped.Load(); dropped != 1 {
		t.Fatalf("expected one dropped record, got %d", dropped)
	}
}

func TestAuditWriterFlushesByTimerAndBatchSize(t *testing.T) {
	t.Run("timer", func(t *testing.T) {
		database := openAuditTestDB(t)
		writer := newAuditWriter(database, auditQueueCapacity)
		go writer.run()
		defer writer.stopForTest()

		writer.enqueue(model.AuditLog{Time: 1, Role: "admin", IP: "127.0.0.1", Path: "/timer", Success: true, Detail: "timer"})
		waitForAuditCount(t, database, 1)
	})

	t.Run("batch size", func(t *testing.T) {
		database := openAuditTestDB(t)
		writer := newAuditWriter(database, auditQueueCapacity)
		go writer.run()
		defer writer.stopForTest()

		for i := 0; i < auditBatchSize; i++ {
			if !writer.enqueue(model.AuditLog{Time: int64(i + 1), Role: "admin", IP: "127.0.0.1", Path: "/batch", Success: true, Detail: "batch"}) {
				t.Fatalf("record %d was unexpectedly dropped", i)
			}
		}
		waitForAuditCount(t, database, auditBatchSize)
	})
}

func TestAuditWriterRetriesFailedBatchThreeTimes(t *testing.T) {
	database := openAuditTestDB(t)
	attempts := 0
	if err := database.Callback().Create().Before("gorm:create").Register("audit:test_failure", func(tx *gorm.DB) {
		attempts++
		tx.AddError(errors.New("forced audit write failure"))
	}); err != nil {
		t.Fatalf("register failure callback: %v", err)
	}

	writer := newAuditWriter(database, auditQueueCapacity)
	writer.writeBatch([]model.AuditLog{{
		Time:    1,
		Role:    "admin",
		IP:      "127.0.0.1",
		Path:    "/retry",
		Success: false,
		Detail:  "retry",
	}})
	if attempts != 3 {
		t.Fatalf("expected 3 write attempts, got %d", attempts)
	}

	var count int64
	if err := database.Model(&model.AuditLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count audit records: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected failed batch to be dropped, got %d records", count)
	}
}

func TestListAuditLogsFiltersAndPaginates(t *testing.T) {
	database := openAuditTestDB(t)
	oldAuditDB := db.AuditDB
	db.AuditDB = database
	t.Cleanup(func() { db.AuditDB = oldAuditDB })

	records := []model.AuditLog{
		{Time: 100, Role: "admin", IP: "10.0.0.1", Path: "/plugins/enable", Success: true, Detail: "启用插件: alpha"},
		{Time: 200, Role: "guest", IP: "10.0.0.2", Path: "/self-service/generate", Success: false, Detail: "申请自助授权码"},
		{Time: 300, Role: "admin", IP: "10.0.0.3", Path: "/remove", Success: false, Detail: "删除地图文件: test.vpk"},
	}
	if err := database.Create(&records).Error; err != nil {
		t.Fatalf("seed audit records: %v", err)
	}

	items, total, err := ListAuditLogs(AuditListFilter{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("list audit records: %v", err)
	}
	if total != 3 || len(items) != 2 || items[0].Time != 300 || items[1].Time != 200 {
		t.Fatalf("unexpected pagination result: total=%d items=%+v", total, items)
	}

	failed := false
	items, total, err = ListAuditLogs(AuditListFilter{
		Page:      1,
		PageSize:  20,
		StartTime: 250,
		EndTime:   350,
		Role:      "admin",
		IP:        "0.0.3",
		Path:      "remove",
		Success:   &failed,
		Keyword:   "地图",
	})
	if err != nil {
		t.Fatalf("filter audit records: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Detail != "删除地图文件: test.vpk" {
		t.Fatalf("unexpected filtered result: total=%d items=%+v", total, items)
	}
}
