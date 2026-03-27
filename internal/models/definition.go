package models

import (
	"time"

	"gorm.io/gorm"
)

type Definition struct {
	ID            string         `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TerminologyID string         `gorm:"type:uuid;not null;index" json:"terminology_id"`
	LocaleID      string         `gorm:"type:uuid;not null;index" json:"locale_id"`
	Translation   string         `gorm:"type:text;not null" json:"translation"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	CreatedBy     *string        `gorm:"type:uuid;index" json:"created_by,omitempty"`
	UpdatedBy     *string        `gorm:"type:uuid;index" json:"updated_by,omitempty"`

	// Relationships
	Terminology Terminology `gorm:"foreignKey:TerminologyID" json:"terminology"`
	Locale      Locale      `gorm:"foreignKey:LocaleID" json:"locale"`
	Creator     *User       `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Updater     *User       `gorm:"foreignKey:UpdatedBy" json:"updater,omitempty"`
}

func (Definition) TableName() string {
	return "definitions"
}

// GORM Hooks for automatic history tracking
func (d *Definition) AfterCreate(tx *gorm.DB) error {
	return recordDefinitionHistory(tx, d, "created")
}

func (d *Definition) AfterUpdate(tx *gorm.DB) error {
	return recordDefinitionHistory(tx, d, "updated")
}

func (d *Definition) AfterDelete(tx *gorm.DB) error {
	return recordDefinitionHistory(tx, d, "deleted")
}

func recordDefinitionHistory(tx *gorm.DB, d *Definition, action string) error {
	history := DefinitionHistory{
		DefinitionID:  d.ID,
		LocaleID:      d.LocaleID,
		TerminologyID: d.TerminologyID,
		UserID:        d.UpdatedBy,
		Translation:   d.Translation,
		Action:        action,
		ChangedAt:     time.Now(),
	}
	return tx.Create(&history).Error
}
