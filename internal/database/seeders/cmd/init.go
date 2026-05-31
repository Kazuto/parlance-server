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

	// Register seeder
	roleSeeder := models.NewRoleSeeder()
	if err := factory.Register(roleSeeder); err != nil {
		return err
	}

	userSeeder := models.NewUserSeeder()
	if err := factory.Register(userSeeder); err != nil {
		return err
	}

	scopeSeeder := models.NewScopeSeeder()
	if err := factory.Register(scopeSeeder); err != nil {
		return err
	}

	entriesSeeder := models.NewEntriesSeeder()
	if err := factory.Register(entriesSeeder); err != nil {
		return err
	}

	localizationsSeeder := models.NewLocalizationsSeeder()
	if err := factory.Register(localizationsSeeder); err != nil {
		return err
	}

	terminologiesSeeder := models.NewTerminologySeeder()
	if err := factory.Register(terminologiesSeeder); err != nil {
		return err
	}

	definitionsSeeder := models.NewDefinitionSeeder()
	if err := factory.Register(definitionsSeeder); err != nil {
		return err
	}

	// Run all seeder
	if err := factory.RunAll(db); err != nil {
		return err
	}

	// Print summary of what was initialized
	for _, seeder := range factory.All() {
		println(seeder.Name(), "-", seeder.Description())
	}

	log.Println("✅ Database seeding completed successfully!")

	return nil
}
