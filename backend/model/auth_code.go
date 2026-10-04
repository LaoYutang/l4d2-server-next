package model

import "time"

// AuthCode contains no plaintext credential. Status is derived at request time.
type AuthCode struct {
	ID           string     `json:"id" gorm:"primaryKey;size:36"`
	Fingerprint  string     `json:"-" gorm:"uniqueIndex;size:64;not null"`
	MaskedCode   string     `json:"masked_code"`
	Remark       string     `json:"remark"`
	AccessType   string     `json:"access_type"`
	Source       string     `json:"source"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at" gorm:"index"`
	RevokedAt    *time.Time `json:"revoked_at"`
	FirstLoginAt *time.Time `json:"first_login_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	LoginCount   int64      `json:"login_count"`
	LastLoginIP  string     `json:"last_login_ip"`
}

type AuthCodeState struct {
	ID                  uint `gorm:"primaryKey"`
	LastSelfServiceTime time.Time
}
