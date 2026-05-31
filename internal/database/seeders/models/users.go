package models

import (
	"log"

	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

type UserSeeder struct{}

func NewUserSeeder() *UserSeeder { return &UserSeeder{} }

func (i *UserSeeder) Name() string { return "users" }
func (i *UserSeeder) Description() string {
	return "Seed users"
}

func (i *UserSeeder) Run(db *gorm.DB) error {
	log.Println("Seeding users...")

	var adminRole, translatorRole, viewerRole models.Role
	db.Where("name = ?", "admin").First(&adminRole)
	db.Where("name = ?", "translator").First(&translatorRole)
	db.Where("name = ?", "viewer").First(&viewerRole)

	users := []struct {
		email    string
		password string
		name     string
		roles    []models.Role
	}{
		{"admin@parlance.dev", "admin123", "Admin User", []models.Role{adminRole}},
		{"translator@parlance.dev", "translator123", "Translator User", []models.Role{translatorRole}},
		{"viewer@parlance.dev", "viewer123", "Viewer User", []models.Role{viewerRole}},
		{"john@example.com", "password123", "John Doe", []models.Role{translatorRole}},
		{"jane@example.com", "password123", "Jane Smith", []models.Role{translatorRole}},
	}

	for _, userData := range users {
		hashedPassword, _ := auth.HashPassword(userData.password)
		user := models.User{
			Email:        userData.email,
			PasswordHash: hashedPassword,
			Name:         userData.name,
		}

		if err := db.Create(&user).Error; err != nil {
			return err
		}

		if err := db.Model(&user).Association("Roles").Replace(&userData.roles); err != nil {
			return err
		}

		log.Printf("Created user: %s (password: %s)", userData.email, userData.password)
	}

	return nil
}

func (i *UserSeeder) Dependencies() []string { return []string{"roles"} }
