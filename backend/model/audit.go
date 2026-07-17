package model

// AuditLog stores one security-sensitive or state-changing manager operation.
// ID is intentionally excluded from JSON and is only used for stable pagination.
type AuditLog struct {
	ID      uint   `json:"-" gorm:"primaryKey"`
	Time    int64  `json:"time" gorm:"index:idx_audit_time"`
	Role    string `json:"role" gorm:"index:idx_audit_role"`
	IP      string `json:"ip" gorm:"index:idx_audit_ip"`
	Path    string `json:"path" gorm:"index:idx_audit_path"`
	Success bool   `json:"success" gorm:"index:idx_audit_success"`
	Detail  string `json:"detail"`
}
