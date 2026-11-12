package models

import "time"

// ReleaseEntry puts an entry in a release. Links stay once the release is out, as a
// record of what it shipped.
type ReleaseEntry struct {
	ReleaseID string    `gorm:"type:uuid;primaryKey" json:"release_id"`
	EntryID   string    `gorm:"type:uuid;primaryKey;index" json:"entry_id"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy *string   `gorm:"type:uuid" json:"created_by,omitempty"`

	// Relationships, for their foreign keys: GORM's default ones for the join table
	// wouldn't cascade.
	Release *Release `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Entry   *Entry   `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

func (ReleaseEntry) TableName() string {
	return "release_entries"
}
