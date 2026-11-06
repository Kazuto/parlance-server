package models

import "time"

type EntryScope struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	EntryID   string    `gorm:"type:uuid;not null;index" json:"entry_id"`
	ScopeID   string    `gorm:"type:uuid;not null;index" json:"scope_id"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy *string   `gorm:"type:uuid" json:"created_by,omitempty"`

	// Relationships
	Entry Entry `gorm:"foreignKey:EntryID" json:"entry"`
	Scope Scope `gorm:"foreignKey:ScopeID" json:"scope"`
}

func (EntryScope) TableName() string {
	return "entry_scopes"
}
