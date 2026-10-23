package models

import "time"

// TranslationServer is an OpenAI-compatible server running a translation model.
// Machine translation spreads its requests over all enabled servers.
type TranslationServer struct {
	ID      string `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Name    string `gorm:"type:varchar(100);not null" json:"name"`
	BaseURL string `gorm:"type:text;not null" json:"base_url"`
	Model   string `gorm:"type:varchar(200)" json:"model"`
	// APIKey is encrypted with the server's encryption key, see package secret.
	APIKey         string    `gorm:"type:text" json:"-"`
	MaxConcurrency int       `gorm:"default:2;not null" json:"max_concurrency"`
	Enabled        bool      `gorm:"not null" json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	CreatedBy      *string   `gorm:"type:uuid" json:"created_by,omitempty"`
	UpdatedBy      *string   `gorm:"type:uuid" json:"updated_by,omitempty"`
}

func (TranslationServer) TableName() string {
	return "translation_servers"
}
