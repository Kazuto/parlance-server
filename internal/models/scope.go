package models

import (
	"time"

	"gorm.io/gorm"
)

type Scope struct {
	ID          string         `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Name        string         `gorm:"type:varchar(100);not null" json:"name"`
	Slug        string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"slug"`
	Description string         `gorm:"type:text" json:"description"`
	Color       string         `gorm:"type:varchar(7)" json:"color"` // Hex color code
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Entries []Entry `gorm:"many2many:entry_scopes;" json:"entries"`
}

func (Scope) TableName() string {
	return "scopes"
}
