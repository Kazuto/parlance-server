package permission

import (
	"testing"
)

// TestListPermissionsSearchMatchesNameOrResource tests PostgreSQL-specific ILIKE
// Skipped with SQLite test database
func TestListPermissionsSearchMatchesNameOrResource(t *testing.T) {
	t.Skip("SQL generation test requires PostgreSQL (tests ILIKE operator)")
}
