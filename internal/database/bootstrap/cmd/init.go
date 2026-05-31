package cmd

import (
	bootstrap "github.com/kazuto/parlance-server/internal/database/bootstrap"
	models "github.com/kazuto/parlance-server/internal/database/bootstrap/models"
	"gorm.io/gorm"
)

func Execute(db *gorm.DB) error {
	factory := bootstrap.NewInitializerFactory()

	// Register initializers
	permissionInit := models.NewPermissionInitializer()
	if err := factory.Register(permissionInit); err != nil {
		return err
	}

	roleInit := models.NewRoleInitializer()
	if err := factory.Register(roleInit); err != nil {
		return err
	}

	localeInit := models.NewLocaleInitializer()
	if err := factory.Register(localeInit); err != nil {
		return err
	}

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
