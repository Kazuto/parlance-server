package models

import (
	"time"

	"gorm.io/gorm"
)

type Localization struct {
	ID          string         `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	EntryID     string         `gorm:"type:uuid;not null;index" json:"entry_id"`
	LocaleID    string         `gorm:"type:uuid;not null;index" json:"locale_id"`
	Translation string         `gorm:"type:text;not null" json:"translation"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	CreatedBy   *string        `gorm:"type:uuid;index" json:"created_by,omitempty"`
	UpdatedBy   *string        `gorm:"type:uuid;index" json:"updated_by,omitempty"`

	// Relationships
	Entry   Entry  `gorm:"foreignKey:EntryID" json:"entry"`
	Locale  Locale `gorm:"foreignKey:LocaleID" json:"locale"`
	Creator *User  `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Updater *User  `gorm:"foreignKey:UpdatedBy" json:"updater,omitempty"`
}

func (Localization) TableName() string {
	return "localizations"
}

// GORM Hooks for automatic history tracking
func (l *Localization) AfterCreate(tx *gorm.DB) error {
	return recordLocalizationHistory(tx, l, "created")
}

func (l *Localization) AfterUpdate(tx *gorm.DB) error {
	return recordLocalizationHistory(tx, l, "updated")
}

func (l *Localization) AfterDelete(tx *gorm.DB) error {
	return recordLocalizationHistory(tx, l, "deleted")
}

func recordLocalizationHistory(tx *gorm.DB, l *Localization, action string) error {
	history := LocalizationHistory{
		LocalizationID: l.ID,
		LocaleID:       l.LocaleID,
		EntryID:        l.EntryID,
		UserID:         l.UpdatedBy,
		Translation:    l.Translation,
		Action:         action,
		ChangedAt:      time.Now(),
	}
	return tx.Create(&history).Error
}
