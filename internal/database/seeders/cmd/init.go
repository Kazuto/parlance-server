package cmd

import (
	"log"

	bootstrap "github.com/kazuto/parlance-server/internal/database/seeders"
	models "github.com/kazuto/parlance-server/internal/database/seeders/models"
	"gorm.io/gorm"
)

func Execute(db *gorm.DB) error {
	log.Println("🌱 Starting database seeding...")

	factory := bootstrap.NewSeederFactory()

	roleSeeder := models.NewRoleSeeder()
	if err := factory.Run(roleSeeder, db); err != nil {
		return err
	}

	userSeeder := models.NewUserSeeder()
	if err := factory.Run(userSeeder, db); err != nil {
		return err
	}

	scopeSeeder := models.NewScopeSeeder()
	if err := factory.Run(scopeSeeder, db); err != nil {
		return err
	}

	entriesSeeder := models.NewEntriesSeeder()
	if err := factory.Run(entriesSeeder, db); err != nil {
		return err
	}

	localizationsSeeder := models.NewLocalizationsSeeder()
	if err := factory.Run(localizationsSeeder, db); err != nil {
		return err
	}

	terminologiesSeeder := models.NewTerminologySeeder()
	if err := factory.Run(terminologiesSeeder, db); err != nil {
		return err
	}

	definitionsSeeder := models.NewDefinitionSeeder()
	if err := factory.Run(definitionsSeeder, db); err != nil {
		return err
	}

	log.Println("✅ Database seeding completed successfully!")

	return nil
}
