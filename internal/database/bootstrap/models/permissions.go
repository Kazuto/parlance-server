package models

import (
	"context"
	"fmt"
	"log"

	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
)

type PermissionInitializer struct{}

func NewPermissionInitializer() *PermissionInitializer { return &PermissionInitializer{} }

func (i *PermissionInitializer) Name() string { return "Permissions" }
func (i *PermissionInitializer) Description() string {
	return "Populates Permissions table"
}

func (i *PermissionInitializer) Run(ctx context.Context) error {
	log.Println("Initializing permissions")
	cfg := config.Load()
	db, err := database.Connect(cfg.Database)
	if err != nil {
		return err
	}

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
		if err := db.FirstOrCreate(&permissions[i], models.Permission{Name: permissions[i].Name}).Error; err != nil {
			return err
		}
	}

	return nil
}

func (i *PermissionInitializer) Dependencies() []string { return []string{"Permissions"} }
