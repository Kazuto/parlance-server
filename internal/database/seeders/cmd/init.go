package cmd

import (
	bootstrap "github.com/kazuto/parlance-server/internal/database/seeders"
	models "github.com/kazuto/parlance-server/internal/database/seeders/models"
	"gorm.io/gorm"
)

type InitCommand struct{}

func NewInitCommand() *InitCommand {
	return &InitCommand{}
}

// Name returns the command name.
func (c *InitCommand) Name() string {
	return "init"
}

func (c *InitCommand) Description() string {
	return "Run all database seeder with fake data"
}

func (c *InitCommand) Run(db *gorm.DB) error {
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

	return nil
}

// Dependencies returns any external dependencies required by this command.
func (c *InitCommand) Dependencies() []string {
	return []string{"database", "users", "roles", "content"}
}
