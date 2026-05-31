package models

import (
	"log"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

type EntriesSeeder struct{}

func NewEntriesSeeder() *EntriesSeeder { return &EntriesSeeder{} }

func (i *EntriesSeeder) Name() string { return "entries" }
func (i *EntriesSeeder) Description() string {
	return "Seed entries"
}

func (i *EntriesSeeder) Run(db *gorm.DB) error {
	log.Println("Seeding entries...")

	// Get admin user for CreatedBy
	var adminUser models.User
	db.Where("email = ?", "admin@parlance.dev").First(&adminUser)

	// Get scopes
	var frontendScope, backendScope, mobileScope models.Scope
	db.Where("name = ?", "frontend").First(&frontendScope)
	db.Where("name = ?", "backend").First(&backendScope)
	db.Where("name = ?", "mobile").First(&mobileScope)

	entries := []struct {
		key         string
		description string
		scopes      []models.Scope
	}{
		{"app.name", "Application name", []models.Scope{frontendScope, mobileScope}},
		{"app.tagline", "Application tagline", []models.Scope{frontendScope}},
		{"button.submit", "Submit button label", []models.Scope{frontendScope, mobileScope}},
		{"button.cancel", "Cancel button label", []models.Scope{frontendScope, mobileScope}},
		{"button.save", "Save button label", []models.Scope{frontendScope, mobileScope}},
		{"button.delete", "Delete button label", []models.Scope{frontendScope, mobileScope}},
		{"nav.home", "Navigation: Home", []models.Scope{frontendScope, mobileScope}},
		{"nav.settings", "Navigation: Settings", []models.Scope{frontendScope, mobileScope}},
		{"nav.profile", "Navigation: Profile", []models.Scope{frontendScope, mobileScope}},
		{"error.not_found", "404 error message", []models.Scope{frontendScope, backendScope}},
		{"error.unauthorized", "401 error message", []models.Scope{frontendScope, backendScope}},
		{"error.server_error", "500 error message", []models.Scope{frontendScope, backendScope}},
		{"login.title", "Login page title", []models.Scope{frontendScope, mobileScope}},
		{"login.email", "Email field label", []models.Scope{frontendScope, mobileScope}},
		{"login.password", "Password field label", []models.Scope{frontendScope, mobileScope}},
		{"login.forgot_password", "Forgot password link", []models.Scope{frontendScope, mobileScope}},
		{"validation.required", "Field required validation message", []models.Scope{frontendScope, backendScope, mobileScope}},
		{"validation.email", "Invalid email validation message", []models.Scope{frontendScope, backendScope, mobileScope}},
		{"notification.success", "Generic success notification", []models.Scope{frontendScope, mobileScope}},
		{"notification.error", "Generic error notification", []models.Scope{frontendScope, mobileScope}},
	}

	for _, entryData := range entries {
		entry := models.Entry{
			Key:         entryData.key,
			Description: entryData.description,
			CreatedBy:   &adminUser.ID,
		}

		if err := db.Create(&entry).Error; err != nil {
			return err
		}

		if len(entryData.scopes) > 0 {
			db.Model(&entry).Association("Scopes").Replace(&entryData.scopes)
		}

		log.Printf("    Created entry: %s", entryData.key)
	}

	return nil
}

func (i *EntriesSeeder) Dependencies() []string { return []string{"users", "scopes"} }
