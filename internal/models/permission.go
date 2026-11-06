package models

import (
	"time"

	"gorm.io/gorm"
)

type Permission struct {
	ID          string         `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Name        string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"name"`
	Resource    string         `gorm:"type:varchar(50);not null" json:"resource"`
	Action      string         `gorm:"type:varchar(50);not null" json:"action"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (Permission) TableName() string {
	return "permissions"
}
