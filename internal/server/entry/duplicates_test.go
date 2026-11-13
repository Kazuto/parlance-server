package entry

import (
	"slices"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestListDuplicateEntries(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}

	testutil.Exec(t, db, `INSERT INTO locales (id, code, is_default, deleted_at) VALUES
		('dup-en', 'dup-en', 1, NULL), ('dup-de', 'dup-de', 0, NULL), ('dup-fr', 'dup-fr', 0, NULL),
		('dup-gone', 'dup-gone', 0, '2026-01-01 00:00:00')`)
	testutil.Exec(t, db, `INSERT INTO scopes (id, name, slug) VALUES ('dup-front', 'Front', 'dup-front'), ('dup-back', 'Back', 'dup-back')`)
	testutil.Exec(t, db, `INSERT INTO entries (id, key, deleted_at) VALUES
		('dup-1', 'dup.a.save', NULL), ('dup-2', 'dup.b.save', NULL), ('dup-3', 'dup.c.save', NULL),
		('dup-4', 'dup.cancel.one', NULL), ('dup-5', 'dup.cancel.two', NULL),
		('dup-6', 'dup.deleted', '2026-01-01 00:00:00'),
		('dup-7', 'dup.blank', NULL), ('dup-8', 'dup.blank.two', NULL),
		('dup-9', 'dup.unique', NULL), ('dup-10', 'dup.unique.gone', NULL)`)
	testutil.Exec(t, db, `INSERT INTO entry_scopes (id, entry_id, scope_id) VALUES
		('dup-s1', 'dup-1', 'dup-front'), ('dup-s2', 'dup-2', 'dup-front'), ('dup-s3', 'dup-3', 'dup-back'),
		('dup-s4', 'dup-4', 'dup-front'), ('dup-s5', 'dup-5', 'dup-back')`)
	testutil.Exec(t, db, `INSERT INTO releases (id, name) VALUES ('dup-rel', 'dup-rel')`)
	testutil.Exec(t, db, `INSERT INTO release_entries (release_id, entry_id) VALUES ('dup-rel', 'dup-1'), ('dup-rel', 'dup-3')`)
	// The save texts differ by case and space only; their German translations differ
	// in words for a and b, by case for a and c, and their French ones by case and
	// space. The cancel texts differ only in a deleted locale.
	testutil.Exec(t, db, `INSERT INTO localizations (id, entry_id, locale_id, translation, deleted_at) VALUES
		('dup-l1', 'dup-1', 'dup-en', 'Dup save', NULL),
		('dup-l2', 'dup-1', 'dup-de', 'Speichern', NULL),
		('dup-l3', 'dup-1', 'dup-fr', 'Enregistrer', NULL),
		('dup-l4', 'dup-2', 'dup-en', '  dup SAVE ', NULL),
		('dup-l5', 'dup-2', 'dup-de', 'Sichern', NULL),
		('dup-l6', 'dup-2', 'dup-fr', 'enregistrer ', NULL),
		('dup-l7', 'dup-3', 'dup-en', 'Dup save', NULL),
		('dup-l8', 'dup-3', 'dup-de', 'speichern', NULL),
		('dup-l9', 'dup-4', 'dup-en', 'Dup cancel', NULL),
		('dup-l10', 'dup-4', 'dup-de', 'Abbrechen', NULL),
		('dup-l11', 'dup-4', 'dup-gone', 'X', NULL),
		('dup-l12', 'dup-5', 'dup-en', 'Dup cancel', NULL),
		('dup-l13', 'dup-5', 'dup-de', 'abbrechen', NULL),
		('dup-l14', 'dup-5', 'dup-gone', 'Y', NULL),
		('dup-l15', 'dup-6', 'dup-en', 'Dup save', NULL),
		('dup-l16', 'dup-7', 'dup-en', '   ', NULL),
		('dup-l17', 'dup-8', 'dup-en', '', NULL),
		('dup-l18', 'dup-9', 'dup-en', 'Dup unique', NULL),
		('dup-l19', 'dup-10', 'dup-en', 'Dup unique', '2026-01-01 00:00:00')`)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM localizations WHERE id LIKE 'dup-%'`)
		db.Exec(`DELETE FROM release_entries WHERE release_id LIKE 'dup-%'`)
		db.Exec(`DELETE FROM releases WHERE id LIKE 'dup-%'`)
		db.Exec(`DELETE FROM entry_scopes WHERE id LIKE 'dup-%'`)
		db.Exec(`DELETE FROM entries WHERE id LIKE 'dup-%'`)
		db.Exec(`DELETE FROM scopes WHERE id LIKE 'dup-%'`)
		db.Exec(`DELETE FROM locales WHERE id LIKE 'dup-%'`)
	})

	keys := func(entries []*pb.Entry) []string {
		keys := make([]string, len(entries))
		for i, entry := range entries {
			keys[i] = entry.Key
		}

		return keys
	}

	list := func(msg *pb.ListEntriesRequest) []string {
		t.Helper()

		res, err := server.ListEntries(ctx, connect.NewRequest(msg))
		if err != nil {
			t.Fatalf("list entries: %v", err)
		}

		return keys(res.Msg.Entries)
	}

	same := pb.DuplicateFilter_DUPLICATE_FILTER_SAME_TEXT
	different := pb.DuplicateFilter_DUPLICATE_FILTER_TRANSLATED_DIFFERENTLY
	allSaves := []string{"dup.a.save", "dup.b.save", "dup.c.save"}

	cases := []struct {
		name string
		msg  *pb.ListEntriesRequest
		want []string
	}{
		{
			name: "same text, by text and then key",
			msg:  &pb.ListEntriesRequest{Duplicates: same},
			want: []string{"dup.cancel.one", "dup.cancel.two", "dup.a.save", "dup.b.save", "dup.c.save"},
		},
		{
			name: "twins within the scope",
			msg:  &pb.ListEntriesRequest{Duplicates: same, ScopeIds: []string{"dup-front"}},
			want: []string{"dup.a.save", "dup.b.save"},
		},
		{
			name: "twins within the release",
			msg:  &pb.ListEntriesRequest{Duplicates: same, ReleaseIds: []string{"dup-rel"}},
			want: []string{"dup.a.save", "dup.c.save"},
		},
		{
			name: "translated differently in German",
			msg:  &pb.ListEntriesRequest{Duplicates: different, StatusLocaleIds: []string{"dup-de"}},
			want: allSaves,
		},
		{
			name: "translated alike in French, ignoring case and space",
			msg:  &pb.ListEntriesRequest{Duplicates: different, StatusLocaleIds: []string{"dup-fr"}},
			want: []string{},
		},
		{
			name: "translated differently in an active locale",
			msg:  &pb.ListEntriesRequest{Duplicates: different},
			want: allSaves,
		},
		{
			name: "translated differently within the scope",
			msg:  &pb.ListEntriesRequest{Duplicates: different, ScopeIds: []string{"dup-front"}, TargetLocaleIds: []string{"dup-de"}},
			want: []string{"dup.a.save", "dup.b.save"},
		},
		{
			name: "twins missing French",
			msg:  &pb.ListEntriesRequest{Duplicates: same, TranslationStatus: "missing", StatusLocaleIds: []string{"dup-fr"}},
			want: []string{"dup.cancel.one", "dup.cancel.two", "dup.c.save"},
		},
		{
			name: "twins found by search",
			msg:  &pb.ListEntriesRequest{Duplicates: same, Filter: &pb.FilterRequest{Search: "dup.b"}},
			want: []string{"dup.b.save"},
		},
	}

	for _, c := range cases {
		if got := list(c.msg); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}

	// Pages count the twins, and keep listing them together.
	res, err := server.ListEntries(ctx, connect.NewRequest(&pb.ListEntriesRequest{
		Duplicates: same,
		Pagination: &pb.PaginationRequest{Page: 2, PerPage: 2},
	}))
	if err != nil {
		t.Fatalf("list page: %v", err)
	}

	if got := keys(res.Msg.Entries); !slices.Equal(got, []string{"dup.a.save", "dup.b.save"}) || res.Msg.Pagination.GetTotal() != 5 {
		t.Errorf("page 2: got %v of %d", got, res.Msg.Pagination.GetTotal())
	}

	// The cursor walks across the groups without skipping or repeating an entry.
	var walked []string

	cursor := ""

	for range 10 {
		res, err := server.ListEntries(ctx, connect.NewRequest(&pb.ListEntriesRequest{
			Duplicates: same,
			Cursor:     cursor,
			Pagination: &pb.PaginationRequest{PerPage: 1},
		}))
		if err != nil {
			t.Fatalf("list after %q: %v", cursor, err)
		}

		walked = append(walked, keys(res.Msg.Entries)...)

		cursor = res.Msg.NextCursor
		if cursor == "" {
			break
		}
	}

	if want := []string{"dup.cancel.one", "dup.cancel.two", "dup.a.save", "dup.b.save", "dup.c.save"}; !slices.Equal(walked, want) {
		t.Errorf("cursor walk: got %v, want %v", walked, want)
	}
}

func TestDuplicatesValidation(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}

	same := pb.DuplicateFilter_DUPLICATE_FILTER_SAME_TEXT
	cases := map[string]*pb.ListEntriesRequest{
		"unknown filter":   {Duplicates: pb.DuplicateFilter(99)},
		"deleted entries":  {Duplicates: same, DeletedOnly: true},
		"key cursor":       {Duplicates: same, Cursor: encodeCursor("dup.a.save")},
		"malformed cursor": {Duplicates: same, Cursor: "%%%"},
		"cursor with sort": {Duplicates: same, Cursor: encodeDuplicateCursor("dup save", "dup.a.save"), Filter: &pb.FilterRequest{Sort: "updated_at"}},
	}

	for name, msg := range cases {
		_, err := server.ListEntries(ctx, connect.NewRequest(msg))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: expected invalid argument, got %v", name, err)
		}
	}

	text, key, ok := decodeDuplicateCursor(encodeDuplicateCursor("dup save", "dup.a.save"))
	if !ok || text != "dup save" || key != "dup.a.save" {
		t.Errorf("round trip: got %q, %q, %v", text, key, ok)
	}
}
