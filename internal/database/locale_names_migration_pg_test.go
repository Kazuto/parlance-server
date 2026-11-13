package database_test

import (
	"testing"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestMigrationDropsLocaleNames(t *testing.T) {
	db := testutil.SetupPostgresDB(t)

	// A database from before locales were named by their codes: each held its names,
	// which new rows had to give.
	testutil.Exec(t, db, `ALTER TABLE locales ADD COLUMN names jsonb NOT NULL DEFAULT '{}'`)
	testutil.Exec(t, db, `ALTER TABLE locales ALTER COLUMN names DROP DEFAULT`)
	testutil.Exec(t, db, `INSERT INTO locales (id, code, names) VALUES (gen_random_uuid(), 'xx', '{"en":"Old"}')`)

	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	if db.Migrator().HasColumn(&models.Locale{}, "names") {
		t.Error("locales still have names")
	}

	var codes []string

	db.Model(&models.Locale{}).Order("code").Pluck("code", &codes)

	if len(codes) != 1 || codes[0] != "xx" {
		t.Errorf("locales = %v, want the existing one kept", codes)
	}

	// New locales need no names, and a second migration leaves the table alone.
	if err := db.Create(&models.Locale{Code: "sv"}).Error; err != nil {
		t.Fatalf("create a locale: %v", err)
	}

	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("second AutoMigrate() error = %v", err)
	}
}
