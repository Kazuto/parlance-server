package terminology

import (
	"testing"
)

// TestListTerminologiesSearchMatchesTermOrDescription tests PostgreSQL-specific ILIKE
// Skipped with SQLite test database
func TestListTerminologiesSearchMatchesTermOrDescription(t *testing.T) {
	t.Skip("SQL generation test requires PostgreSQL (tests ILIKE operator)")
}
