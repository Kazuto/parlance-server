package models

import (
	"log"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

type TerminologySeeder struct{}

func NewTerminologySeeder() *TerminologySeeder { return &TerminologySeeder{} }

func (i *TerminologySeeder) Name() string { return "terminologies" }
func (i *TerminologySeeder) Description() string {
	return "Seed terminologies"
}

func (i *TerminologySeeder) Run(db *gorm.DB) error {
	log.Println("Seeding terminologies...")

	var adminUser models.User
	db.Where("email = ?", "admin@parlance.dev").First(&adminUser)

	terminologies := []struct {
		term        string
		description string
	}{
		{"Logo", "Company or product logo/branding"},
		{"Dashboard", "Main application dashboard view"},
		{"User", "Application user or account holder"},
		{"Email", "Email address or email communication"},
		{"Password", "Authentication password"},
		{"Settings", "Application settings and configuration"},
		{"Profile", "User profile information"},
		{"API", "Application Programming Interface"},
		{"Login", "Authentication/sign-in action"},
		{"Logout", "Sign-out action"},
	}

	for _, termData := range terminologies {
		terminology := models.Terminology{
			Term:        termData.term,
			Description: termData.description,
			CreatedBy:   &adminUser.ID,
		}

		if err := db.Create(&terminology).Error; err != nil {
			return err
		}

		log.Printf("    Created terminology: %s", termData.term)
	}

	return nil
}

func (i *TerminologySeeder) Dependencies() []string { return []string{"users"} }
