package models

import (
	"fmt"
	"log"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

type ScopeSeeder struct{}

func NewScopeSeeder() *ScopeSeeder { return &ScopeSeeder{} }

func (i *ScopeSeeder) Name() string { return "scopes" }
func (i *ScopeSeeder) Description() string {
	return "Seed scopes"
}

func (i *ScopeSeeder) Run(db *gorm.DB) error {
	log.Println("Seeding scopes...")

	scopes := []models.Scope{
		{Name: "Frontend", Slug: "frontend", Description: "Frontend UI translations", Color: "#3B82F6"},
		{Name: "Backend", Slug: "backend", Description: "Backend messages and errors", Color: "#10B981"},
		{Name: "Validation", Slug: "validation", Description: "Form validation messages", Color: "#F59E0B"},
		{Name: "Emails", Slug: "emails", Description: "Email templates and notifications", Color: "#8B5CF6"},
		{Name: "Common", Slug: "common", Description: "Common shared translations", Color: "#6B7280"},
	}

	if err := db.Create(&scopes).Error; err != nil {
		return fmt.Errorf("failed to seed scopes: %w", err)
	}

	return nil
}

func (i *ScopeSeeder) Dependencies() []string { return []string{} }
