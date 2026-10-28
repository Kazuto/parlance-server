package cmd

import (
	bootstrap "github.com/kazuto/parlance-server/internal/database/bootstrap"
	models "github.com/kazuto/parlance-server/internal/database/bootstrap/models"
	"gorm.io/gorm"
)

func Execute(db *gorm.DB) error {
	factory := bootstrap.NewInitializerFactory()

	// Register initializers - only the minimum required for setup
	// Permissions and admin role are necessary for the first admin user
	permissionInit := models.NewPermissionInitializer()
	if err := factory.Register(permissionInit); err != nil {
		return err
	}

	roleInit := models.NewRoleInitializer()
	if err := factory.Register(roleInit); err != nil {
		return err
	}

	// Note: Locales are NOT seeded during setup - users create them as needed

	// Run all initializers
	if err := factory.RunAll(db); err != nil {
		return err
	}

	// Print summary of what was initialized
	for _, initializer := range factory.All() {
		println(initializer.Name(), "-", initializer.Description())
	}

	return nil
}
