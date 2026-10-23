package models

import (
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

type ScopeSeeder struct{}

func NewScopeSeeder() *ScopeSeeder { return &ScopeSeeder{} }

func (i *ScopeSeeder) Name() string { return "Scopes" }

func (i *ScopeSeeder) Run(db *gorm.DB) error {
	scopes := []models.Scope{
		{Name: "Frontend", Slug: "frontend", Description: "Frontend UI translations", Color: "#3B82F6"},
		{Name: "Backend", Slug: "backend", Description: "Backend messages and errors", Color: "#10B981"},
		{Name: "Validation", Slug: "validation", Description: "Form validation messages", Color: "#F59E0B"},
		{Name: "Emails", Slug: "emails", Description: "Email templates and notifications", Color: "#8B5CF6"},
		{Name: "Common", Slug: "common", Description: "Common shared translations", Color: "#6B7280"},
		{Name: "Mobile", Slug: "mobile", Description: "Mobile UI translations", Color: "#3B82F6"},
	}

	for _, s := range scopes {
		db.Where("name = ?", s.Name).FirstOrCreate(&s)
	}

	return nil
}

func (i *ScopeSeeder) Dependencies() []string { return []string{} }
