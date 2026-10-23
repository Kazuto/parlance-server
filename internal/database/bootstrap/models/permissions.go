package models

import (
	"fmt"
	"log"
	"strings"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"gorm.io/gorm"
)

type PermissionInitializer struct{}

func NewPermissionInitializer() *PermissionInitializer { return &PermissionInitializer{} }

func (i *PermissionInitializer) Name() string { return "Permissions" }
func (i *PermissionInitializer) Description() string {
	return "Populates Permissions table"
}

func (i *PermissionInitializer) Run(db *gorm.DB) error {
	log.Println("Initializing permissions")

	var perms []models.Permission

	// Use all permissions from the constants
	for _, permName := range permissions.AllPermissions() {
		// Parse permission name to extract resource and action
		// Format: action_resource (e.g., "create_locale")
		parts := strings.SplitN(permName, "_", 2)
		if len(parts) != 2 {
			log.Printf("Warning: Invalid permission name format: %s", permName)

			continue
		}

		action := parts[0]
		resource := parts[1]

		// Handle multi-word actions like "permanent_delete_locale"
		if strings.HasPrefix(permName, "permanent_delete_") {
			action = "permanent_delete"
			resource = strings.TrimPrefix(permName, "permanent_delete_")
		}

		perms = append(perms, models.Permission{
			Name:        permName,
			Resource:    resource,
			Action:      action,
			Description: fmt.Sprintf("Allows the user to %s %s resources", action, resource),
		})
	}

	for i := range perms {
		var existing int64
		if err := db.Model(&models.Permission{}).Where("name = ?", perms[i].Name).Count(&existing).Error; err != nil {
			return err
		}

		if existing > 0 {
			continue
		}

		if err := db.Create(&perms[i]).Error; err != nil {
			return err
		}

		if err := grantToEquivalentRoles(db, perms[i]); err != nil {
			return fmt.Errorf("failed to grant %s: %w", perms[i].Name, err)
		}
	}

	return nil
}

// grantToEquivalentRoles gives a new permission to the roles that could already do
// what it covers (see permissions.GrantedWith), so splitting it off takes nothing away.
func grantToEquivalentRoles(db *gorm.DB, permission models.Permission) error {
	required, ok := permissions.GrantedWith[permission.Name]
	if !ok {
		return nil
	}

	var roleIDs []string
	if err := db.Table("role_permissions").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("permissions.name IN ?", required).
		Group("role_permissions.role_id").
		Having("COUNT(DISTINCT permissions.name) = ?", len(required)).
		Pluck("role_permissions.role_id", &roleIDs).Error; err != nil {
		return err
	}

	for _, roleID := range roleIDs {
		if err := db.Model(&models.Role{ID: roleID}).Association("Permissions").Append(&permission); err != nil {
			return err
		}

		log.Printf("Granted %s to role %s, which held %s", permission.Name, roleID, strings.Join(required, " and "))
	}

	return nil
}

func (i *PermissionInitializer) Dependencies() []string { return []string{"Permissions"} }
