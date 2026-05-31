package database

import (
	"fmt"
	"log"

	"github.com/kazuto/parlance-server/internal/config"
	bootstrap "github.com/kazuto/parlance-server/internal/database/bootstrap/cmd"
	seeder "github.com/kazuto/parlance-server/internal/database/seeders/cmd"
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

func (db *DB) Bootstrap() error {
	bootstrap.Execute(db.DB)

	return nil
}

// Seed inserts default data
func (db *DB) Seed() error {
	seeder.Execute(db.DB)

	return nil
}
