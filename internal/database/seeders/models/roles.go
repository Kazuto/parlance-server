package models

import (
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

type RoleSeeder struct{}

func NewRoleSeeder() *RoleSeeder { return &RoleSeeder{} }

func (i *RoleSeeder) Name() string { return "Roles" }

func (i *RoleSeeder) Run(db *gorm.DB) error {
	roles := []models.Role{
		{Name: "admin", Description: "Full system access"},
		{Name: "translator", Description: "Can create and edit translations"},
		{Name: "reviewer", Description: "Can review and approve translations"},
		{Name: "viewer", Description: "Read-only access"},
	}

	for _, r := range roles {
		db.Where("name = ?", r.Name).FirstOrCreate(&r)
	}

	// Assign permissions to roles
	var adminRole models.Role
	db.Where("name = ?", "admin").Preload("Permissions").First(&adminRole)

	var allPermissions []models.Permission
	db.Find(&allPermissions)
	db.Model(&adminRole).Association("Permissions").Append(&allPermissions)

	var translatorRole models.Role
	db.Where("name = ?", "translator").First(&translatorRole)

	var translatorPerms []models.Permission
	db.Where("name IN ?", []string{
		"entry.read", "entry.create", "entry.update",
		"localization.read", "localization.create", "localization.update", "localization.translate",
		"terminology.read", "terminology.create", "terminology.update",
		"scope.read", "export.use",
	}).Find(&translatorPerms)

	db.Model(&translatorRole).Association("Permissions").Append(&translatorPerms)

	var reviewerRole models.Role
	db.Where("name = ?", "reviewer").First(&reviewerRole)

	var reviewerPerms []models.Permission
	db.Where("name IN ?", []string{
		"entry.read", "entry.update",
		"localization.read", "localization.update",
		"terminology.read", "scope.read", "export.use",
	}).Find(&reviewerPerms)

	db.Model(&reviewerRole).Association("Permissions").Append(&reviewerPerms)

	var viewerRole models.Role
	db.Where("name = ?", "viewer").First(&viewerRole)

	var viewerPerms []models.Permission
	db.Where("name IN ?", []string{
		"entry.read", "localization.read", "terminology.read", "scope.read", "export.use",
	}).Find(&viewerPerms)

	db.Model(&viewerRole).Association("Permissions").Append(&viewerPerms)

	return nil
}

func (i *RoleSeeder) Dependencies() []string { return []string{"permissions"} }
