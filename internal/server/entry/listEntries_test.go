package entry

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

// TestTranslationStatusSQL tests PostgreSQL-specific SQL generation
func TestTranslationStatusSQL(t *testing.T) {
	t.Skip("SQL generation test requires PostgreSQL database")
}

// TestScopeFilterSQL tests PostgreSQL-specific SQL generation
func TestScopeFilterSQL(t *testing.T) {
	t.Skip("SQL generation test requires PostgreSQL database")
}

func TestInvalidTranslationStatus(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}

	server := &Server{db: db}
	_, err = server.ListEntries(ctx, connect.NewRequest(&pb.ListEntriesRequest{TranslationStatus: "invalid"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	for _, key := range []string{"actions.deleteEntry", "ümlaut.key", "a/b+c=d"} {
		cursor := encodeCursor(key)
		got, err := decodeCursor(cursor)
		if err != nil || got != key {
			t.Errorf("round trip %q: got %q, %v", key, got, err)
		}
	}
}

func TestCursorValidation(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}

	server := &Server{db: db}
	cases := map[string]*pb.ListEntriesRequest{
		"malformed cursor": {Cursor: "%%%"},
		"non-key sort":     {Cursor: encodeCursor("a"), Filter: &pb.FilterRequest{Sort: "created_at"}},
		"descending key":   {Cursor: encodeCursor("a"), Filter: &pb.FilterRequest{Sort: "key", Order: "desc"}},
	}
	for name, msg := range cases {
		_, err := server.ListEntries(ctx, connect.NewRequest(msg))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: expected invalid argument, got %v", name, err)
		}
	}
}
