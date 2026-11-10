package logic

import (
	"context"
	"l4d2-manager-next/model"
	"time"

	"gorm.io/gorm"
)

const (
	auditRetention        = 30 * 24 * time.Hour
	auditCleanupInterval  = 24 * time.Hour
	auditCleanupBatchSize = 1000
	auditCleanupTimeout   = 5 * time.Second
)

// deleteExpiredAuditBatch uses AuditLog.Time (operation start, Unix seconds).
// The exact cutoff is retained. A subquery bounds each DELETE on SQLite, which
// does not support GORM's DELETE LIMIT, and uses the existing time index.
func deleteExpiredAuditBatch(ctx context.Context, database *gorm.DB, cutoff int64) (int64, error) {
	if database == nil {
		return 0, ErrAuditDatabaseUnavailable
	}
	query := database.WithContext(ctx)
	expired := query.Model(&model.AuditLog{}).Select("id").
		Where("time < ?", cutoff).Order("time").Order("id").Limit(auditCleanupBatchSize)
	result := query.Where("id IN (?)", expired).Delete(&model.AuditLog{})
	return result.RowsAffected, result.Error
}
