package models

import (
	"context"
	"log"

	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
)

type RoleInitializer struct{}

func NewRoleInitializer() *RoleInitializer { return &RoleInitializer{} }

func (i *RoleInitializer) Name() string { return "roles" }
func (i *RoleInitializer) Description() string {
	return "Populates roles table"
}

func (i *RoleInitializer) Run(ctx context.Context) error {
	log.Println("Initializing admin role and attach permissions...")
	cfg := config.Load()
	db, err := database.Connect(cfg.Database)
	if err != nil {
		return err
	}

	var permissions []models.Permission
	if err := db.Find(&permissions).Error; err != nil {
		return err
	}

	admin := models.Role{Name: "admin", Permissions: permissions}
	if err := db.FirstOrCreate(&admin, models.Role{Name: "admin"}).Error; err != nil {
		return err
	}

	db.Model(&admin).Association("Permissions").Replace(&permissions)

	return nil
}

func (i *RoleInitializer) Dependencies() []string { return []string{"roles"} }
