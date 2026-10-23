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
	Review        `gorm:"embedded"`

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
	// Bulk statements (Where(...).Delete(&Model{})) carry no row; callers that need
	// history load the rows and pass them instead.
	if d.ID == "" {
		return nil
	}

	userID := d.UpdatedBy

	switch action {
	case "created":
		userID = d.CreatedBy
	case ActionApproved, ActionRejected:
		userID = d.ReviewedBy
	}

	comment := ""
	if action == ActionRejected {
		comment = d.ReviewComment
	}

	history := DefinitionHistory{
		DefinitionID:  d.ID,
		LocaleID:      d.LocaleID,
		TerminologyID: d.TerminologyID,
		UserID:        userID,
		Translation:   d.Translation,
		Action:        action,
		Status:        d.Status,
		Comment:       comment,
		ChangedAt:     time.Now(),
	}

	return tx.Create(&history).Error
}

// RecordReview writes the history of a review, which the review saves without hooks
func (d *Definition) RecordReview(tx *gorm.DB, action string) error {
	return recordDefinitionHistory(tx, d, action)
}

// RowID returns the id, for code handling reviewable rows of any kind
func (d *Definition) RowID() string {
	return d.ID
}
