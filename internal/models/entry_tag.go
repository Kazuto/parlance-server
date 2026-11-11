package models

import "time"

// EntryTag puts an entry under a tag of one of its scopes. ScopeID repeats the tag's
// scope so the database can check that the entry is in it: removing the scope from
// the entry removes the entry's tags of that scope.
//
// Write rows directly instead of through Entry's Tags association, which would leave
// ScopeID empty.
type EntryTag struct {
	EntryID   string    `gorm:"type:uuid;primaryKey" json:"entry_id"`
	TagID     string    `gorm:"type:uuid;primaryKey;index" json:"tag_id"`
	ScopeID   string    `gorm:"type:uuid;not null" json:"scope_id"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy *string   `gorm:"type:uuid" json:"created_by,omitempty"`

	// Relationships, for their foreign keys: GORM's default ones for the join table
	// wouldn't cascade.
	Entry *Entry `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Tag   *Tag   `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

func (EntryTag) TableName() string {
	return "entry_tags"
}
