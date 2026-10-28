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
	Review      `gorm:"embedded"`
	// When the default locale's text first changed after this text was written or
	// approved; nil while it fits the current source text. Set apart from the review,
	// so an approved text can be outdated.
	OutdatedAt *time.Time `json:"outdated_at,omitempty"`
	// Origin tells whether a person or a model wrote the text, and MachineModel
	// which model, as its server names it. See WrittenBy.
	Origin       string `gorm:"type:varchar(20);not null;default:'human'" json:"origin"`
	MachineModel string `gorm:"type:varchar(200);not null;default:''" json:"machine_model,omitempty"`

	// Relationships
	Entry   Entry  `gorm:"foreignKey:EntryID" json:"entry"`
	Locale  Locale `gorm:"foreignKey:LocaleID" json:"locale"`
	Creator *User  `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Updater *User  `gorm:"foreignKey:UpdatedBy" json:"updater,omitempty"`
}

// Origins of a translation's text
const (
	OriginHuman   = "human"
	OriginMachine = "machine"
)

// WrittenBy records who wrote the current text: the model, or a person when model
// is empty. Any edit by a person makes the text theirs.
func (l *Localization) WrittenBy(model string) {
	if model == "" {
		l.Origin = OriginHuman
		l.MachineModel = ""

		return
	}

	l.Origin = OriginMachine
	l.MachineModel = model
}

// IsMachine tells whether a model wrote the current text.
func (l *Localization) IsMachine() bool {
	return l.Origin == OriginMachine
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
	// Bulk statements (Where(...).Delete(&Model{})) carry no row; callers that need
	// history load the rows and pass them instead.
	if l.ID == "" {
		return nil
	}

	userID := l.UpdatedBy

	switch action {
	case "created":
		userID = l.CreatedBy
	case ActionApproved, ActionRejected:
		userID = l.ReviewedBy
	}

	comment := ""
	if action == ActionRejected {
		comment = l.ReviewComment
	}

	history := LocalizationHistory{
		LocalizationID: l.ID,
		LocaleID:       l.LocaleID,
		EntryID:        l.EntryID,
		UserID:         userID,
		Translation:    l.Translation,
		Action:         action,
		Status:         l.Status,
		Comment:        comment,
		ChangedAt:      time.Now(),
	}

	return tx.Create(&history).Error
}

// RecordReview writes the history of a review, which the review saves without hooks
func (l *Localization) RecordReview(tx *gorm.DB, action string) error {
	return recordLocalizationHistory(tx, l, action)
}

// ClearOutdated marks the text as fitting the current source text again
func (l *Localization) ClearOutdated() {
	l.OutdatedAt = nil
}

// RowID returns the id, for code handling reviewable rows of any kind
func (l *Localization) RowID() string {
	return l.ID
}
