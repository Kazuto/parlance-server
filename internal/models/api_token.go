package models

import "time"

// APIToken lets scripts and CI call the API as the user who created it, with that
// user's permissions. Only a hash of the token is stored; revoking deletes the row.
type APIToken struct {
	ID     string `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	UserID string `gorm:"type:uuid;not null;index" json:"user_id"`
	User   *User  `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Name   string `gorm:"type:varchar(100);not null" json:"name"`
	// TokenHash is the token's SHA-256 hash, which requests are matched by.
	TokenHash string `gorm:"type:char(64);not null;uniqueIndex" json:"-"`
	// Prefix is the token's start, which tells its owner which one it is.
	Prefix     string     `gorm:"type:varchar(16);not null" json:"prefix"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	// ExpiresAt is unset for a token that never expires.
	ExpiresAt *time.Time `gorm:"index" json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (APIToken) TableName() string {
	return "api_tokens"
}
