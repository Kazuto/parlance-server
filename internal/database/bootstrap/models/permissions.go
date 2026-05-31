package models

import (
	"fmt"
	"log"

	"github.com/kazuto/parlance-server/internal/models"
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

	actions := []string{"create", "read", "update", "delete"}
	resources := []string{"definition", "entry", "locale", "Permission", "scope", "terminology", "user"}
	var permissions []models.Permission

	for _, r := range resources {
		for _, a := range actions {
			permissions = append(permissions, models.Permission{
				Name:        fmt.Sprintf("%s_%s", a, r),
				Resource:    r,
				Action:      a,
				Description: fmt.Sprintf("Allows the user to %s %s resources", a, r),
			})
		}
	}

	for i := range permissions {
		db.Where("name = ?", permissions[i].Name).FirstOrCreate(&permissions[i])
	}

	return nil
}

func (i *PermissionInitializer) Dependencies() []string { return []string{"Permissions"} }
