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
		db.Where("name = ?", perms[i].Name).FirstOrCreate(&perms[i])
	}

	return nil
}

func (i *PermissionInitializer) Dependencies() []string { return []string{"Permissions"} }
