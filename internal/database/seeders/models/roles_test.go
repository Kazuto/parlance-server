package models_test

import (
	"sort"
	"testing"

	"github.com/google/uuid"

	seeders "github.com/kazuto/parlance-server/internal/database/seeders/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestRoleSeederAssignsExistingPermissions(t *testing.T) {
	db, err := testutil.SetupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test db: %v", err)
	}
	// SQLite doesn't generate ids for these rows, so create them up front; the seeder
	// then finds them by name, as it would after bootstrap in PostgreSQL.
	for _, name := range permissions.AllPermissions() {
		db.Exec("INSERT INTO permissions (id, name, resource, action) VALUES (?, ?, '', '')", uuid.NewString(), name)
	}

	for _, name := range []string{"admin", "translator", "reviewer", "viewer"} {
		db.Exec("INSERT INTO roles (id, name) VALUES (?, ?)", uuid.NewString(), name)
	}

	// Seeding twice must leave each role with exactly its permissions.
	for range 2 {
		if err := seeders.NewRoleSeeder().Run(db.DB); err != nil {
			t.Fatalf("RoleSeeder: %v", err)
		}
	}

	want := map[string]int{"translator": 12, "reviewer": 7, "viewer": 5}
	for role, count := range want {
		var names []string

		db.Raw(`SELECT p.name FROM permissions p
			JOIN role_permissions rp ON rp.permission_id = p.id
			JOIN roles r ON r.id = rp.role_id
			WHERE r.name = ?`, role).Scan(&names)
		sort.Strings(names)

		if len(names) != count {
			t.Errorf("%s: got %d permissions %v, want %d", role, len(names), names, count)
		}

		if i := sort.SearchStrings(names, permissions.ReadEntry); i >= len(names) || names[i] != permissions.ReadEntry {
			t.Errorf("%s: missing %s in %v", role, permissions.ReadEntry, names)
		}
	}
}
