package scope

import (
	"testing"
)

// TestListScopesSearchMatchesNameOrSlug tests PostgreSQL-specific ILIKE
// Skipped with SQLite test database
func TestListScopesSearchMatchesNameOrSlug(t *testing.T) {
	t.Skip("SQL generation test requires PostgreSQL (tests ILIKE operator)")
}
