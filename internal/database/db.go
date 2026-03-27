package database

import (
	"fmt"
	"log"

	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	*gorm.DB
}

func Connect(cfg config.DatabaseConfig) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		cfg.SSLMode,
	)

	// Configure GORM logger
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB for connection pooling
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	log.Println("Database connection established")

	return &DB{db}, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// AutoMigrate runs database migrations
func (db *DB) AutoMigrate() error {
	log.Println("Running database migrations...")

	// Enable UUID extension
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		return fmt.Errorf("failed to create uuid extension: %w", err)
	}

	// Run migrations for all models
	err := db.DB.AutoMigrate(
		&models.Locale{},
		&models.Scope{},
		&models.User{},
		&models.Role{},
		&models.Permission{},
		&models.Entry{},
		&models.EntryScope{},
		&models.Localization{},
		&models.LocalizationHistory{},
		&models.Terminology{},
		&models.Definition{},
		&models.DefinitionHistory{},
	)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Add unique constraints that GORM doesn't handle well
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_locales_default ON locales (is_default) WHERE is_default = true AND deleted_at IS NULL")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_localizations_entry_locale ON localizations (entry_id, locale_id) WHERE deleted_at IS NULL")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_definitions_term_locale ON definitions (terminology_id, locale_id) WHERE deleted_at IS NULL")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_entry_scopes_unique ON entry_scopes (entry_id, scope_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_user_roles_unique ON user_roles (user_id, role_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_role_permissions_unique ON role_permissions (role_id, permission_id)")

	log.Println("Database migrations completed")

	return nil
}

// Seed inserts default data
func (db *DB) Seed() error {
	log.Println("Seeding database with default data...")

	// Check if already seeded
	var count int64
	db.Model(&models.Locale{}).Count(&count)

	if count > 0 {
		log.Println("Database already seeded, skipping...")

		return nil
	}

	// Seed locales
	locales := []models.Locale{
		{Code: "en", Name: "English", IsDefault: true},
		{Code: "de", Name: "German", IsDefault: false},
		{Code: "fr", Name: "French", IsDefault: false},
		{Code: "es", Name: "Spanish", IsDefault: false},
		{Code: "it", Name: "Italian", IsDefault: false},
		{Code: "pt", Name: "Portuguese", IsDefault: false},
		{Code: "ja", Name: "Japanese", IsDefault: false},
		{Code: "zh", Name: "Chinese", IsDefault: false},
	}

	if err := db.Create(&locales).Error; err != nil {
		return fmt.Errorf("failed to seed locales: %w", err)
	}

	// Seed scopes
	scopes := []models.Scope{
		{Name: "Frontend", Slug: "frontend", Description: "Frontend UI translations", Color: "#3B82F6"},
		{Name: "Backend", Slug: "backend", Description: "Backend messages and errors", Color: "#10B981"},
		{Name: "Validation", Slug: "validation", Description: "Form validation messages", Color: "#F59E0B"},
		{Name: "Emails", Slug: "emails", Description: "Email templates and notifications", Color: "#8B5CF6"},
		{Name: "Common", Slug: "common", Description: "Common shared translations", Color: "#6B7280"},
	}

	if err := db.Create(&scopes).Error; err != nil {
		return fmt.Errorf("failed to seed scopes: %w", err)
	}

	// Seed roles
	roles := []models.Role{
		{Name: "admin", Description: "Full system access"},
		{Name: "translator", Description: "Can create and edit translations"},
		{Name: "reviewer", Description: "Can review and approve translations"},
		{Name: "viewer", Description: "Read-only access"},
	}

	if err := db.Create(&roles).Error; err != nil {
		return fmt.Errorf("failed to seed roles: %w", err)
	}

	// Seed permissions
	permissions := []models.Permission{
		// Entry permissions
		{Name: "entry.read", Resource: "entry", Action: "read", Description: "View translation entries"},
		{Name: "entry.create", Resource: "entry", Action: "create", Description: "Create new translation entries"},
		{Name: "entry.update", Resource: "entry", Action: "update", Description: "Update translation entries"},
		{Name: "entry.delete", Resource: "entry", Action: "delete", Description: "Delete translation entries"},

		// Localization permissions
		{Name: "localization.read", Resource: "localization", Action: "read", Description: "View translations"},
		{Name: "localization.create", Resource: "localization", Action: "create", Description: "Create translations"},
		{Name: "localization.update", Resource: "localization", Action: "update", Description: "Update translations"},
		{Name: "localization.delete", Resource: "localization", Action: "delete", Description: "Delete translations"},
		{Name: "localization.translate", Resource: "localization", Action: "translate", Description: "Use AI translation"},

		// Terminology permissions
		{Name: "terminology.read", Resource: "terminology", Action: "read", Description: "View glossary terms"},
		{Name: "terminology.create", Resource: "terminology", Action: "create", Description: "Create glossary terms"},
		{Name: "terminology.update", Resource: "terminology", Action: "update", Description: "Update glossary terms"},
		{Name: "terminology.delete", Resource: "terminology", Action: "delete", Description: "Delete glossary terms"},

		// Scope permissions
		{Name: "scope.read", Resource: "scope", Action: "read", Description: "View scopes"},
		{Name: "scope.manage", Resource: "scope", Action: "manage", Description: "Manage scopes"},

		// User permissions
		{Name: "user.read", Resource: "user", Action: "read", Description: "View users"},
		{Name: "user.manage", Resource: "user", Action: "manage", Description: "Manage users"},

		// System permissions
		{Name: "export.use", Resource: "export", Action: "use", Description: "Export translations"},
	}

	if err := db.Create(&permissions).Error; err != nil {
		return fmt.Errorf("failed to seed permissions: %w", err)
	}

	// Assign permissions to roles
	var adminRole models.Role
	db.Where("name = ?", "admin").Preload("Permissions").First(&adminRole)

	var allPermissions []models.Permission
	db.Find(&allPermissions)
	db.Model(&adminRole).Association("Permissions").Append(&allPermissions)

	var translatorRole models.Role
	db.Where("name = ?", "translator").First(&translatorRole)

	var translatorPerms []models.Permission
	db.Where("name IN ?", []string{
		"entry.read", "entry.create", "entry.update",
		"localization.read", "localization.create", "localization.update", "localization.translate",
		"terminology.read", "terminology.create", "terminology.update",
		"scope.read", "export.use",
	}).Find(&translatorPerms)

	db.Model(&translatorRole).Association("Permissions").Append(&translatorPerms)

	var reviewerRole models.Role
	db.Where("name = ?", "reviewer").First(&reviewerRole)

	var reviewerPerms []models.Permission
	db.Where("name IN ?", []string{
		"entry.read", "entry.update",
		"localization.read", "localization.update",
		"terminology.read", "scope.read", "export.use",
	}).Find(&reviewerPerms)

	db.Model(&reviewerRole).Association("Permissions").Append(&reviewerPerms)

	var viewerRole models.Role
	db.Where("name = ?", "viewer").First(&viewerRole)

	var viewerPerms []models.Permission
	db.Where("name IN ?", []string{
		"entry.read", "localization.read", "terminology.read", "scope.read", "export.use",
	}).Find(&viewerPerms)

	db.Model(&viewerRole).Association("Permissions").Append(&viewerPerms)

	log.Println("Database seeding completed")

	return nil
}
