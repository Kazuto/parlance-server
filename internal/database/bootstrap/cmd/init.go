package cmd

import (
	"context"

	bootstrap "github.com/kazuto/parlance-server/internal/database/bootstrap"
	models "github.com/kazuto/parlance-server/internal/database/bootstrap/models"
)

// InitCommand is the CLI command to run all initializers.
type InitCommand struct{}

// NewInitCommand creates a new init command.
func NewInitCommand() *InitCommand {
	return &InitCommand{}
}

// Name returns the command name.
func (c *InitCommand) Name() string {
	return "init"
}

// Description provides human-readable information about the command.
func (c *InitCommand) Description() string {
	return "Run all database initializers with realistic data"
}

// Run executes the command, running all registered initializers.
func (c *InitCommand) Run(ctx context.Context) error {
	factory := bootstrap.NewInitializerFactory()

	// Register initializers
	permissionInit := models.NewPermissionInitializer()
	if err := factory.Register(permissionInit); err != nil {
		return err
	}

	// Register initializers
	roleInit := models.NewRoleInitializer()
	if err := factory.Register(roleInit); err != nil {
		return err
	}

	// Run all initializers
	if err := factory.RunAll(ctx); err != nil {
		return err
	}

	// Print summary of what was initialized
	for _, initializer := range factory.All() {
		println(initializer.Name(), "-", initializer.Description())
	}

	return nil
}

// Dependencies returns any external dependencies required by this command.
func (c *InitCommand) Dependencies() []string {
	return []string{"database", "users", "roles", "content"}
}
