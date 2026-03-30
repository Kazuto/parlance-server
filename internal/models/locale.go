package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Locale struct {
	ID        string         `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Code      string         `gorm:"type:varchar(10);uniqueIndex;not null" json:"code"`
	Names     datatypes.JSON `gorm:"type:jsonb;not null" json:"names"` // {"en": "English", "de": "Englisch", ...}
	IsDefault bool           `gorm:"default:false;not null" json:"is_default"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (Locale) TableName() string {
	return "locales"
}
