package testutil

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/database"
)

// Setup is SetupTestContext for tests: it fails the test when the database cannot be set up
func Setup(t *testing.T) (context.Context, *database.DB) {
	t.Helper()

	ctx, db, err := SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}

	return ctx, db
}

// Exec runs a statement, e.g. to seed rows, failing the test when it does not succeed
func Exec(t *testing.T, db *database.DB, query string, args ...any) {
	t.Helper()

	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// ExpectInvalidArgument calls the handler with each request and fails the test for any the
// handler does not reject as invalid input
func ExpectInvalidArgument[Req, Res any](
	t *testing.T,
	ctx context.Context,
	handler func(context.Context, *connect.Request[Req]) (*connect.Response[Res], error),
	requests ...*Req,
) {
	t.Helper()

	for _, request := range requests {
		if _, err := handler(ctx, connect.NewRequest(request)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%v: expected invalid argument, got %v", request, err)
		}
	}
}
