package testutil

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	perms "github.com/kazuto/parlance-server/internal/permissions"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// uuidHook generates UUIDs for models in tests (SQLite doesn't support uuid_generate_v4).
// It covers single records and slices of them.
func uuidHook(db *gorm.DB) {
	if db.Statement.Schema == nil {
		return
	}
	// Matches uuid columns too: their DataType is "uuid", not "string".
	idField := db.Statement.Schema.LookUpField("ID")
	if idField == nil || idField.FieldType.Kind() != reflect.String {
		return
	}

	ctx := db.Statement.Context
	setID := func(record reflect.Value) {
		if _, isZero := idField.ValueOf(ctx, record); isZero {
			_ = idField.Set(ctx, record, uuid.New().String())
		}
	}

	value := reflect.Indirect(db.Statement.ReflectValue)
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			setID(reflect.Indirect(value.Index(i)))
		}
	case reflect.Struct:
		setID(value)
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
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			name TEXT NOT NULL,
			locale TEXT DEFAULT 'en',
			avatar TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS roles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS permissions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			resource TEXT NOT NULL,
			action TEXT NOT NULL,
			description TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS user_roles (
			user_id TEXT,
			role_id TEXT,
			PRIMARY KEY (user_id, role_id)
		)`,
		`CREATE TABLE IF NOT EXISTS role_permissions (
			role_id TEXT,
			permission_id TEXT,
			PRIMARY KEY (role_id, permission_id)
		)`,
		`CREATE TABLE IF NOT EXISTS entries (
			id TEXT PRIMARY KEY,
			key TEXT NOT NULL UNIQUE,
			description TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			created_by TEXT,
			updated_by TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS definitions (
			id TEXT PRIMARY KEY,
			terminology_id TEXT,
			locale_id TEXT,
			translation TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			created_by TEXT,
			updated_by TEXT,
			status TEXT NOT NULL DEFAULT 'draft',
			reviewed_by TEXT,
			reviewed_at DATETIME,
			review_comment TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS locales (
			id TEXT PRIMARY KEY,
			code TEXT NOT NULL UNIQUE,
			names TEXT,
			is_default INTEGER DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS localizations (
			id TEXT PRIMARY KEY,
			entry_id TEXT,
			locale_id TEXT,
			translation TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			created_by TEXT,
			updated_by TEXT,
			status TEXT NOT NULL DEFAULT 'draft',
			reviewed_by TEXT,
			reviewed_at DATETIME,
			review_comment TEXT NOT NULL DEFAULT ''
		)`,
		// Saving or deleting a localization or definition records it in these tables.
		`CREATE TABLE IF NOT EXISTS localization_history (
			id TEXT PRIMARY KEY,
			localization_id TEXT NOT NULL,
			locale_id TEXT NOT NULL,
			entry_id TEXT NOT NULL,
			user_id TEXT,
			translation TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT '',
			comment TEXT NOT NULL DEFAULT '',
			changed_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS definition_history (
			id TEXT PRIMARY KEY,
			definition_id TEXT NOT NULL,
			locale_id TEXT NOT NULL,
			terminology_id TEXT NOT NULL,
			user_id TEXT,
			translation TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT '',
			comment TEXT NOT NULL DEFAULT '',
			changed_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS scopes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			description TEXT,
			color TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS entry_scopes (
			id TEXT PRIMARY KEY,
			entry_id TEXT,
			scope_id TEXT,
			created_at DATETIME,
			created_by TEXT
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_entry_scopes_unique ON entry_scopes (entry_id, scope_id)`,
		`CREATE TABLE IF NOT EXISTS terminologies (
			id TEXT PRIMARY KEY,
			term TEXT NOT NULL,
			description TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			created_by TEXT,
			updated_by TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS translation_servers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			base_url TEXT NOT NULL,
			model TEXT,
			api_key TEXT,
			max_concurrency INTEGER NOT NULL DEFAULT 2,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME,
			created_by TEXT,
			updated_by TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 0,
			schedule TEXT NOT NULL DEFAULT '',
			next_run_at DATETIME,
			events TEXT,
			event_delay_minutes INTEGER NOT NULL DEFAULT 0,
			config TEXT,
			secret TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			created_by TEXT,
			updated_by TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS task_runs (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			trigger TEXT NOT NULL,
			triggered_by TEXT,
			status TEXT NOT NULL,
			run_after DATETIME NOT NULL,
			cancel_requested INTEGER NOT NULL DEFAULT 0,
			started_at DATETIME,
			heartbeat_at DATETIME,
			finished_at DATETIME,
			summary TEXT,
			log TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			created_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME,
			updated_by TEXT
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_task_runs_queued ON task_runs (task_id) WHERE status = 'queued'`,
		`CREATE TABLE IF NOT EXISTS webhooks (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			events TEXT,
			enabled INTEGER NOT NULL DEFAULT 0,
			format TEXT NOT NULL DEFAULT 'json',
			schedule TEXT NOT NULL DEFAULT '',
			next_digest_at DATETIME,
			secret TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			created_by TEXT,
			updated_by TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS sso_providers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			issuer_url TEXT NOT NULL,
			client_id TEXT NOT NULL,
			client_secret TEXT,
			default_role_id TEXT,
			enabled INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME,
			created_by TEXT,
			updated_by TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS sso_identities (
			id TEXT PRIMARY KEY,
			provider_id TEXT NOT NULL,
			subject TEXT NOT NULL,
			user_id TEXT NOT NULL,
			email TEXT NOT NULL DEFAULT '',
			created_at DATETIME,
			last_login_at DATETIME
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sso_identities_subject ON sso_identities (provider_id, subject)`,
		`CREATE TABLE IF NOT EXISTS sso_logins (
			state TEXT PRIMARY KEY,
			provider_id TEXT NOT NULL,
			nonce TEXT NOT NULL,
			code_verifier TEXT NOT NULL,
			redirect_uri TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS webhook_deliveries (
			id TEXT PRIMARY KEY,
			webhook_id TEXT NOT NULL,
			event_id TEXT NOT NULL,
			event TEXT NOT NULL,
			payload TEXT NOT NULL,
			status TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at DATETIME,
			last_attempt_at DATETIME,
			response_status INTEGER NOT NULL DEFAULT 0,
			response_body TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			duration_ms INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			return nil, fmt.Errorf("create test schema: %w", err)
		}
	}

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
	permissions := make([]models.Permission, 0, len(perms.AllPermissions()))
	for _, name := range perms.AllPermissions() {
		permissions = append(permissions, models.Permission{Name: name})
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

// ContextWithPermissions creates a user holding only the named permissions, which
// must exist, e.g. after SetupTestContext, and returns a context signed in as them.
func ContextWithPermissions(db *database.DB, names ...string) (context.Context, error) {
	var permissions []models.Permission
	if err := db.Where("name IN ?", names).Find(&permissions).Error; err != nil {
		return nil, err
	}

	id := uuid.New().String()

	role := models.Role{ID: id, Name: "role-" + id, Permissions: permissions}
	if err := db.Create(&role).Error; err != nil {
		return nil, err
	}

	user := models.User{
		ID:           id,
		Email:        id + "@example.com",
		Name:         "Limited User",
		PasswordHash: "not-used-in-tests",
		Roles:        []models.Role{role},
	}
	if err := db.Create(&user).Error; err != nil {
		return nil, err
	}

	return ContextWithTestUser(context.Background(), user.ID), nil
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
