package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           string         `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Email        string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"type:varchar(255);not null" json:"-"`
	Name         string         `gorm:"type:varchar(255);not null" json:"name"`
	Locale       *string        `gorm:"type:varchar(255);default:'en'" json:"locale"`
	Avatar       *string        `gorm:"type:varchar(255)" json:"avatar"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relationships
	Roles []Role `gorm:"many2many:user_roles;" json:"roles"`
}

func (User) TableName() string {
	return "users"
}
