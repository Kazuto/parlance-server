package models

import "time"

// Tag groups entries within a scope, e.g. "onboarding" in the "mobile" scope. Names are
// unique within a scope, ignoring case.
type Tag struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	ScopeID   string    `gorm:"type:uuid;not null;index" json:"scope_id"`
	Name      string    `gorm:"type:varchar(100);not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy *string   `gorm:"type:uuid" json:"created_by,omitempty"`
	UpdatedBy *string   `gorm:"type:uuid" json:"updated_by,omitempty"`
}

func (Tag) TableName() string {
	return "tags"
}
