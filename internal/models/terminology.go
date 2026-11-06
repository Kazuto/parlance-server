package models

import (
	"time"

	"gorm.io/gorm"
)

type Terminology struct {
	ID          string         `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Term        string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"term"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	CreatedBy   *string        `gorm:"type:uuid;index" json:"created_by,omitempty"`
	UpdatedBy   *string        `gorm:"type:uuid;index" json:"updated_by,omitempty"`

	// Relationships
	Definitions []Definition `gorm:"foreignKey:TerminologyID" json:"definitions"`
	Creator     *User        `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Updater     *User        `gorm:"foreignKey:UpdatedBy" json:"updater,omitempty"`
}

func (Terminology) TableName() string {
	return "terminologies"
}
