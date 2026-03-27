package models

import "time"

type LocalizationHistory struct {
	ID             string    `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	LocalizationID string    `gorm:"type:uuid;not null;index" json:"localization_id"`
	LocaleID       string    `gorm:"type:uuid;not null" json:"locale_id"`
	EntryID        string    `gorm:"type:uuid;not null;index" json:"entry_id"`
	UserID         *string   `gorm:"type:uuid" json:"user_id,omitempty"`
	Translation    string    `gorm:"type:text;not null" json:"translation"`
	Action         string    `gorm:"type:varchar(20);not null" json:"action"` // created, updated, deleted
	ChangedAt      time.Time `gorm:"not null;index:idx_changed_at,sort:desc" json:"changed_at"`

	// Relationships
	Localization Localization `gorm:"foreignKey:LocalizationID" json:"localization"`
	User         *User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (LocalizationHistory) TableName() string {
	return "localization_history"
}
