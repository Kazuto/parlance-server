package models

import "time"

// Release holds its entries back from exports until it goes out: at ReleaseAt, or when
// someone releases it without a date. Names are unique, ignoring case.
type Release struct {
	ID          string `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Name        string `gorm:"type:varchar(100);not null" json:"name"`
	Description string `gorm:"type:text;not null;default:''" json:"description"`
	// ReleaseAt is when the release goes out; nil holds it until it's released by hand.
	ReleaseAt *time.Time `gorm:"index" json:"release_at,omitempty"`
	// ReleasedAt is when it went out and was announced.
	ReleasedAt *time.Time `gorm:"index" json:"released_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	CreatedBy  *string    `gorm:"type:uuid" json:"created_by,omitempty"`
	UpdatedBy  *string    `gorm:"type:uuid" json:"updated_by,omitempty"`
}

func (Release) TableName() string {
	return "releases"
}

// Pending tells whether the release hasn't gone out yet.
func (r *Release) Pending() bool {
	return r.ReleasedAt == nil
}
