package models

import (
	"time"

	"gorm.io/gorm"
)

type Entry struct {
	ID          string         `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Key         string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"key"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	CreatedBy   *string        `gorm:"type:uuid;index" json:"created_by,omitempty"`
	UpdatedBy   *string        `gorm:"type:uuid;index" json:"updated_by,omitempty"`

	// Relationships
	Localizations []Localization `gorm:"foreignKey:EntryID" json:"localizations"`
	Scopes        []Scope        `gorm:"many2many:entry_scopes;" json:"scopes"`
	Creator       *User          `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Updater       *User          `gorm:"foreignKey:UpdatedBy" json:"updater,omitempty"`

	// Tags joined through EntryTag, see database.SetupJoinTables
	Tags []Tag `gorm:"many2many:entry_tags;" json:"tags"`
	// Releases joined through ReleaseEntry, see database.SetupJoinTables
	Releases []Release `gorm:"many2many:release_entries;" json:"releases"`
}

func (Entry) TableName() string {
	return "entries"
}
