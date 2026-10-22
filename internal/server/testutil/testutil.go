package testutil

import (
	"context"

	"github.com/google/uuid"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// uuidHook generates UUIDs for models in tests (SQLite doesn't support uuid_generate_v4)
func uuidHook(db *gorm.DB) {
	if db.Statement.Schema != nil {
		for _, field := range db.Statement.Schema.Fields {
			if field.Name == "ID" && field.DataType == "string" {
				if idField := db.Statement.Schema.LookUpField("ID"); idField != nil {
					if idValue, ok := idField.ValueOf(db.Statement.Context, db.Statement.ReflectValue); ok {
						if idStr, ok := idValue.(string); ok && idStr == "" {
							_ = idField.Set(db.Statement.Context, db.Statement.ReflectValue, uuid.New().String())
						}
					}
				}
			}
		}
	}
}

// SetupTestDB creates an in-memory SQLite database for testing
func SetupTestDB() (*database.DB, error) {
	// Use SQLite in memory for fast tests
	// Disable foreign keys check during migration
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_foreign_keys=off"), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, err
	}

	// Add UUID generation hook for tests (SQLite doesn't support uuid_generate_v4())
	if err := db.Callback().Create().Before("gorm:create").Register("test:uuid", uuidHook); err != nil {
		return nil, err
	}

	// Manually create tables without PostgreSQL-specific features
	// This is simpler than trying to work around GORM's migrator
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		name TEXT NOT NULL,
		locale TEXT DEFAULT 'en',
		avatar TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS roles (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS permissions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		resource TEXT NOT NULL,
		action TEXT NOT NULL,
		description TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS user_roles (
		user_id TEXT,
		role_id TEXT,
		PRIMARY KEY (user_id, role_id)
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS role_permissions (
		role_id TEXT,
		permission_id TEXT,
		PRIMARY KEY (role_id, permission_id)
	)`)

	// Add other tables as needed for specific tests
	db.Exec(`CREATE TABLE IF NOT EXISTS entries (
		id TEXT PRIMARY KEY,
		key TEXT NOT NULL UNIQUE,
		description TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME,
		created_by TEXT,
		updated_by TEXT
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS definitions (
		id TEXT PRIMARY KEY,
		entry_id TEXT,
		locale_id TEXT,
		translation TEXT NOT NULL,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME,
		created_by TEXT,
		updated_by TEXT
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS locales (
		id TEXT PRIMARY KEY,
		code TEXT NOT NULL UNIQUE,
		names TEXT,
		is_default INTEGER DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS localizations (
		id TEXT PRIMARY KEY,
		entry_id TEXT,
		locale_id TEXT,
		translation TEXT NOT NULL,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME,
		created_by TEXT,
		updated_by TEXT
	)`)

	// Saving or deleting a localization or definition records it in these tables.
	db.Exec(`CREATE TABLE IF NOT EXISTS localization_history (
		id TEXT PRIMARY KEY,
		localization_id TEXT NOT NULL,
		locale_id TEXT NOT NULL,
		entry_id TEXT NOT NULL,
		user_id TEXT,
		translation TEXT NOT NULL,
		action TEXT NOT NULL,
		changed_at DATETIME NOT NULL
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS definition_history (
		id TEXT PRIMARY KEY,
		definition_id TEXT NOT NULL,
		locale_id TEXT NOT NULL,
		terminology_id TEXT NOT NULL,
		user_id TEXT,
		translation TEXT NOT NULL,
		action TEXT NOT NULL,
		changed_at DATETIME NOT NULL
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS scopes (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		slug TEXT NOT NULL UNIQUE,
		description TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS entry_scopes (
		id TEXT PRIMARY KEY,
		entry_id TEXT,
		scope_id TEXT,
		created_at DATETIME,
		created_by TEXT
	)`)
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_entry_scopes_unique ON entry_scopes (entry_id, scope_id)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS terminologies (
		id TEXT PRIMARY KEY,
		term TEXT NOT NULL,
		description TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)

	return &database.DB{DB: db}, nil
}

// CreateTestUserWithAllPermissions creates a test user with all permissions
func CreateTestUserWithAllPermissions(db *database.DB) (string, error) {
	// First check if permissions already exist, delete them to avoid UNIQUE constraint errors
	db.Exec("DELETE FROM user_roles")
	db.Exec("DELETE FROM role_permissions")
	db.Exec("DELETE FROM users")
	db.Exec("DELETE FROM roles")
	db.Exec("DELETE FROM permissions")

	// Create all permissions
	permissions := []models.Permission{
		{Name: "read_entry"},
		{Name: "create_entry"},
		{Name: "update_entry"},
		{Name: "delete_entry"},
		{Name: "read_definition"},
		{Name: "create_definition"},
		{Name: "update_definition"},
		{Name: "delete_definition"},
		{Name: "read_locale"},
		{Name: "create_locale"},
		{Name: "update_locale"},
		{Name: "delete_locale"},
		{Name: "read_localization"},
		{Name: "update_localization"},
		{Name: "read_role"},
		{Name: "create_role"},
		{Name: "update_role"},
		{Name: "delete_role"},
		{Name: "read_permission"},
		{Name: "read_scope"},
		{Name: "create_scope"},
		{Name: "update_scope"},
		{Name: "delete_scope"},
		{Name: "read_terminology"},
		{Name: "create_terminology"},
		{Name: "update_terminology"},
		{Name: "delete_terminology"},
		{Name: "read_user"},
		{Name: "create_user"},
		{Name: "update_user"},
		{Name: "delete_user"},
	}

	for i := range permissions {
		permissions[i].ID = uuid.New().String()
	}

	if err := db.Create(&permissions).Error; err != nil {
		return "", err
	}

	// Create admin role with all permissions
	role := models.Role{
		ID:          uuid.New().String(),
		Name:        "Test Admin",
		Description: "Test admin role with all permissions",
		Permissions: permissions,
	}

	if err := db.Create(&role).Error; err != nil {
		return "", err
	}

	// Create test user
	user := models.User{
		ID:           uuid.New().String(),
		Email:        "test@example.com",
		Name:         "Test User",
		PasswordHash: "not-used-in-tests",
		Roles:        []models.Role{role},
	}

	if err := db.Create(&user).Error; err != nil {
		return "", err
	}

	return user.ID, nil
}

// ContextWithTestUser creates a context with a test user ID
func ContextWithTestUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, middleware.ContextKeyUserID, userID)
}

// SetupTestContext creates a test database and context with an admin user
func SetupTestContext() (context.Context, *database.DB, error) {
	db, err := SetupTestDB()
	if err != nil {
		return nil, nil, err
	}

	userID, err := CreateTestUserWithAllPermissions(db)
	if err != nil {
		return nil, nil, err
	}

	ctx := ContextWithTestUser(context.Background(), userID)
	return ctx, db, nil
}
