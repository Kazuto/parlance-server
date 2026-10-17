package testutil

import (
	"context"
)

// MockContextWithUser creates a context with a mocked user ID for testing
// This allows tests to bypass permission checks
func MockContextWithUser() context.Context {
	// Use a fake but valid-looking UUID
	return context.WithValue(context.Background(), "user_id", "test-user-id-for-unit-tests")
}
