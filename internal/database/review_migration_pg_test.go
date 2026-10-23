package database_test

import (
	"slices"
	"testing"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

func TestMigrationApprovesExistingTranslations(t *testing.T) {
	db := testutil.SetupPostgresDB(t)

	// A database from before review states: the column is missing.
	for _, table := range []string{"localizations", "definitions"} {
		testutil.Exec(t, db, "ALTER TABLE "+table+" DROP COLUMN status")
	}

	locale := models.Locale{Code: "xx", Names: datatypes.JSON(`{"en":"Test"}`)}
	entry := models.Entry{Key: "old"}

	for _, row := range []any{&locale, &entry} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	testutil.Exec(t, db, "INSERT INTO localizations (entry_id, locale_id, translation) VALUES (?, ?, 'Alt')", entry.ID, locale.ID)

	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	var status string

	db.Table("localizations").Select("status").Where("entry_id = ?", entry.ID).Scan(&status)

	if status != models.StatusApproved {
		t.Errorf("existing translation is %q, want approved", status)
	}

	// Later migrations leave the column alone, and new rows are drafts.
	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("second AutoMigrate() error = %v", err)
	}

	added := models.Localization{EntryID: entry.ID, LocaleID: locale.ID, Translation: "Neu"}

	testutil.Exec(t, db, "DELETE FROM localizations")

	if err := db.Create(&added).Error; err != nil {
		t.Fatal(err)
	}

	db.Table("localizations").Select("status").Where("id = ?", added.ID).Scan(&status)

	if status != models.StatusDraft {
		t.Errorf("new translation is %q, want draft", status)
	}
}

func TestBootstrapGrantsSplitPermissionsOnce(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	// A database from before import and export had permissions of their own.
	split := []string{permissions.ImportEntry, permissions.ExportEntry}
	testutil.Exec(t, db, "DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE name IN ?)", split)
	testutil.Exec(t, db, "DELETE FROM permissions WHERE name IN ?", split)

	roles := map[string][]string{
		"editor": {permissions.ReadEntry, permissions.CreateEntry, permissions.UpdateEntry},
		"reader": {permissions.ReadEntry},
		"other":  {permissions.ReadLocale},
	}
	for name, names := range roles {
		var granted []models.Permission
		if err := db.Where("name IN ?", names).Find(&granted).Error; err != nil {
			t.Fatal(err)
		}

		if err := db.Create(&models.Role{Name: name, Permissions: granted}).Error; err != nil {
			t.Fatal(err)
		}
	}

	held := func(role string) []string {
		var names []string

		db.Table("permissions").
			Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
			Joins("JOIN roles ON roles.id = role_permissions.role_id").
			Where("roles.name = ?", role).Order("permissions.name").Pluck("permissions.name", &names)

		return names
	}

	for range 2 {
		if err := db.Bootstrap(); err != nil {
			t.Fatalf("bootstrap: %v", err)
		}
	}

	want := map[string][]string{
		"editor": {"create_entry", "export_entry", "import_entry", "read_entry", "update_entry"},
		"reader": {"export_entry", "read_entry"},
		"other":  {"read_locale"},
	}
	for role, names := range want {
		if got := held(role); !slices.Equal(got, names) {
			t.Errorf("%s holds %v, want %v", role, got, names)
		}
	}
}
