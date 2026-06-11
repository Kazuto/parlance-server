package models

import (
	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

type UserSeeder struct{}

func NewUserSeeder() *UserSeeder { return &UserSeeder{} }

func (i *UserSeeder) Name() string { return "Users" }

func (i *UserSeeder) Run(db *gorm.DB) error {
	var adminRole, translatorRole, viewerRole models.Role
	db.Where("name = ?", "admin").First(&adminRole)
	db.Where("name = ?", "translator").First(&translatorRole)
	db.Where("name = ?", "viewer").First(&viewerRole)

	users := []struct {
		email    string
		password string
		name     string
		locale   string
		roles    []models.Role
	}{
		{"admin@parlance.dev", "admin123", "Admin User", "en", []models.Role{adminRole}},
		{"translator@parlance.dev", "translator123", "Translator User", "en", []models.Role{translatorRole}},
		{"viewer@parlance.dev", "viewer123", "Viewer User", "en", []models.Role{viewerRole}},
		{"john@example.com", "password123", "John Doe", "en", []models.Role{translatorRole}},
		{"jane@example.com", "password123", "Jane Smith", "en", []models.Role{translatorRole}},
	}

	for _, userData := range users {
		hashedPassword, _ := auth.HashPassword(userData.password)
		user := models.User{
			Email:        userData.email,
			PasswordHash: hashedPassword,
			Name:         userData.name,
		}

		db.Where("email = ?", userData.email).FirstOrCreate(&user)

		if err := db.Model(&user).Association("Roles").Replace(&userData.roles); err != nil {
			return err
		}
	}

	return nil
}

func (i *UserSeeder) Dependencies() []string { return []string{"roles"} }
