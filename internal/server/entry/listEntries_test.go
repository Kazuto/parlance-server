package entry

import (
	"strings"
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

func TestSummaryDefaultTranslation(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}
	server := &Server{db: db}

	long := strings.Repeat("é", 300)
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	exec(`INSERT INTO locales (id, code, is_default) VALUES ('dt-en', 'dt-en', 1), ('dt-de', 'dt-de', 0)`)
	exec(`INSERT INTO entries (id, key) VALUES
		('dt-1', 'dt.translated'), ('dt-2', 'dt.only-german'), ('dt-3', 'dt.padded'),
		('dt-4', 'dt.long'), ('dt-5', 'dt.deleted')`)
	exec(`INSERT INTO localizations (id, entry_id, locale_id, translation, deleted_at) VALUES
		('dt-l1', 'dt-1', 'dt-en', 'Hello world', NULL),
		('dt-l2', 'dt-1', 'dt-de', 'Hallo Welt', NULL),
		('dt-l3', 'dt-2', 'dt-de', 'Nur Deutsch', NULL),
		('dt-l4', 'dt-3', 'dt-en', '  Padded  ', NULL),
		('dt-l5', 'dt-4', 'dt-en', ?, NULL),
		('dt-l6', 'dt-5', 'dt-en', 'Removed', '2026-01-01 00:00:00')`, long)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM localizations WHERE id LIKE 'dt-%'`)
		db.Exec(`DELETE FROM entries WHERE id LIKE 'dt-%'`)
		db.Exec(`DELETE FROM locales WHERE id LIKE 'dt-%'`)
	})

	list := func(summary bool) map[string]string {
		t.Helper()
		res, err := server.ListEntries(ctx, connect.NewRequest(&pb.ListEntriesRequest{
			Summary: summary,
			Filter:  &pb.FilterRequest{Search: "dt."},
		}))
		if err != nil {
			t.Fatalf("ListEntries: %v", err)
		}
		got := map[string]string{}
		for _, entry := range res.Msg.Entries {
			got[entry.Key] = entry.DefaultTranslation
		}
		return got
	}

	got := list(true)
	want := map[string]string{
		"dt.translated":  "Hello world",
		"dt.only-german": "",
		"dt.padded":      "Padded",
		"dt.long":        strings.Repeat("é", maxDefaultTranslationLength-1) + "…",
		"dt.deleted":     "",
	}
	for key, expected := range want {
		if got[key] != expected {
			t.Errorf("%s: got %q, want %q", key, got[key], expected)
		}
	}
	if runes := []rune(got["dt.long"]); len(runes) != maxDefaultTranslationLength {
		t.Errorf("dt.long: got %d runes, want %d", len(runes), maxDefaultTranslationLength)
	}

	// Full listings carry the localizations themselves.
	for key, text := range list(false) {
		if text != "" {
			t.Errorf("%s: default_translation set without summary: %q", key, text)
		}
	}

	// Without a default locale there is nothing to preview.
	exec(`UPDATE locales SET is_default = 0 WHERE id = 'dt-en'`)
	if text := list(true)["dt.translated"]; text != "" {
		t.Errorf("without a default locale: got %q, want empty", text)
	}
}
