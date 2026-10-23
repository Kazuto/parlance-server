package models

import (
	"fmt"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
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

	if err := db.Model(&adminRole).Association("Permissions").Append(&allPermissions); err != nil {
		return err
	}

	// Translations (localizations), import and export are covered by the entry
	// permissions; see rbac.Rules.
	rolePermissions := map[string][]string{
		"translator": {
			permissions.ReadLocale,
			permissions.ReadEntry, permissions.CreateEntry, permissions.UpdateEntry,
			permissions.ReadTerminology, permissions.CreateTerminology, permissions.UpdateTerminology,
			permissions.ReadDefinition, permissions.CreateDefinition, permissions.UpdateDefinition,
			permissions.ReadScope,
			permissions.UseMachineTranslation,
		},
		"reviewer": {
			permissions.ReadLocale,
			permissions.ReadEntry, permissions.UpdateEntry,
			permissions.ReadTerminology,
			permissions.ReadDefinition, permissions.UpdateDefinition,
			permissions.ReadScope,
		},
		"viewer": {
			permissions.ReadLocale,
			permissions.ReadEntry,
			permissions.ReadTerminology,
			permissions.ReadDefinition,
			permissions.ReadScope,
		},
	}

	for roleName, names := range rolePermissions {
		var role models.Role
		if err := db.Where("name = ?", roleName).First(&role).Error; err != nil {
			return err
		}

		var perms []models.Permission
		if err := db.Where("name IN ?", names).Find(&perms).Error; err != nil {
			return err
		}

		if len(perms) != len(names) {
			return fmt.Errorf("role %s: found %d of %d permissions", roleName, len(perms), len(names))
		}

		// Replace so reseeding resets the role to exactly this set.
		if err := db.Model(&role).Association("Permissions").Replace(&perms); err != nil {
			return err
		}
	}

	return nil
}

func (i *RoleSeeder) Dependencies() []string { return []string{"permissions"} }
