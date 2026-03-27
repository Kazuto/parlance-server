package models

import "time"

type DefinitionHistory struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	DefinitionID  string    `gorm:"type:uuid;not null;index" json:"definition_id"`
	LocaleID      string    `gorm:"type:uuid;not null" json:"locale_id"`
	TerminologyID string    `gorm:"type:uuid;not null;index" json:"terminology_id"`
	UserID        *string   `gorm:"type:uuid" json:"user_id,omitempty"`
	Translation   string    `gorm:"type:text;not null" json:"translation"`
	Action        string    `gorm:"type:varchar(20);not null" json:"action"` // created, updated, deleted
	ChangedAt     time.Time `gorm:"not null;index:idx_changed_at,sort:desc" json:"changed_at"`

	// Relationships
	Definition Definition `gorm:"foreignKey:DefinitionID" json:"definition"`
	User       *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (DefinitionHistory) TableName() string {
	return "definition_history"
}
