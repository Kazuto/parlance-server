package testutil

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kazuto/parlance-server/internal/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgresDSNEnv names the variable holding a Postgres DSN for integration tests, e.g.
// "host=localhost port=5432 user=test password=test dbname=test sslmode=disable".
const PostgresDSNEnv = "PARLANCE_TEST_DATABASE_DSN"

// SetupPostgresDB migrates a fresh schema in the Postgres database named by
// PARLANCE_TEST_DATABASE_DSN, which is dropped when the test ends. Tests needing real
// constraints, hooks or SQL features SQLite lacks use it; they are skipped when it is unset.
func SetupPostgresDB(t *testing.T) *database.DB {
	t.Helper()

	dsn := os.Getenv(PostgresDSNEnv)
	if dsn == "" {
		t.Skipf("%s not set", PostgresDSNEnv)
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}

	if err := admin.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`).Error; err != nil {
		t.Fatalf("create uuid extension: %v", err)
	}

	schema := "test_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")

		if sqlDB, err := admin.DB(); err == nil {
			sqlDB.Close()
		}
	})

	gormDB, err := gorm.Open(postgres.Open(fmt.Sprintf("%s search_path=%s,public", dsn, schema)), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("connect to test schema: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := gormDB.DB(); err == nil {
			sqlDB.Close()
		}
	})

	db := &database.DB{DB: gormDB}
	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return db
}
