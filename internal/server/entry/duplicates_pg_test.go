package entry

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestSimilarAndDuplicateEntries(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "read_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	en := models.Locale{Code: "xa", IsDefault: true}
	de := models.Locale{Code: "xb"}

	for _, locale := range []*models.Locale{&en, &de} {
		if err := db.Create(locale).Error; err != nil {
			t.Fatal(err)
		}
	}

	entries := map[string]string{}
	add := func(key, source, german string) {
		t.Helper()

		entry := models.Entry{Key: key}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}

		entries[key] = entry.ID
		rows := []models.Localization{{EntryID: entry.ID, LocaleID: en.ID, Translation: source}}

		if german != "" {
			rows = append(rows, models.Localization{EntryID: entry.ID, LocaleID: de.ID, Translation: german})
		}

		if err := db.Create(&rows).Error; err != nil {
			t.Fatal(err)
		}
	}

	long := strings.Repeat("a", similarMaxLength+1)

	// Similarity to "Save your changes": 1 for the first three, 0.81 and 0.76.
	add("save.exact", "Save your changes", "Änderungen speichern")
	add("save.case", "  save YOUR changes ", "Speichere Änderungen")
	add("save.editing", "Save your changes", "")
	add("save.now", "Save your changes now", "")
	add("save.all", "Save all changes", "")
	add("save.deleted", "Save your changes", "")
	// Short texts are twins too.
	add("ok.one", "OK", "")
	add("ok.two", "ok", "")
	add("long.one", long, "")
	add("long.two", long, "")

	if err := db.Delete(&models.Entry{}, "id = ?", entries["save.deleted"]).Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{db: db}
	search := func(req *pb.SearchSimilarEntriesRequest) ([]string, int32) {
		t.Helper()

		res, err := server.SearchSimilarEntries(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("SearchSimilarEntries(%v) error = %v", req, err)
		}

		keys := make([]string, len(res.Msg.Entries))
		for i, entry := range res.Msg.Entries {
			keys[i] = entry.Key
		}

		return keys, res.Msg.SameTextCount
	}
	check := func(name string, req *pb.SearchSimilarEntriesRequest, count int32, want ...string) {
		t.Helper()

		got, sameText := search(req)
		if !slices.Equal(got, want) || sameText != count {
			t.Errorf("%s: entries %v with %d of the same text, want %v with %d", name, got, sameText, want, count)
		}
	}

	editing := entries["save.editing"]
	check("best first", &pb.SearchSimilarEntriesRequest{Text: "Save your changes", ExcludeEntryId: editing},
		2, "save.case", "save.exact", "save.now")
	check("limited, still counting", &pb.SearchSimilarEntriesRequest{Text: "Save your changes", ExcludeEntryId: editing, Limit: 1},
		2, "save.case")
	check("not excluding", &pb.SearchSimilarEntriesRequest{Text: "save your changes "},
		3, "save.case", "save.editing", "save.exact", "save.now")
	check("short", &pb.SearchSimilarEntriesRequest{Text: "OK", ExcludeEntryId: entries["ok.one"]}, 1, "ok.two")
	check("long, counted only", &pb.SearchSimilarEntriesRequest{Text: long, ExcludeEntryId: entries["long.one"]}, 1)
	check("blank", &pb.SearchSimilarEntriesRequest{Text: "  "}, 0)

	res, err := server.SearchSimilarEntries(ctx, connect.NewRequest(&pb.SearchSimilarEntriesRequest{Text: "Save your changes", ExcludeEntryId: editing}))
	if err != nil {
		t.Fatal(err)
	}

	if now := res.Msg.Entries[2]; now.Text != "Save your changes now" || now.EntryId != entries["save.now"] || now.Similarity < 0.8 || now.Similarity > 0.82 {
		t.Errorf("save.now = %+v, want about 0.81 alike", now)
	}

	// The duplicates filter groups the same texts on Postgres too, and its cursor walks them.
	var walked []string

	cursor := ""

	for range 10 {
		res, err := server.ListEntries(ctx, connect.NewRequest(&pb.ListEntriesRequest{
			Duplicates: pb.DuplicateFilter_DUPLICATE_FILTER_SAME_TEXT,
			Cursor:     cursor,
			Summary:    true,
			Pagination: &pb.PaginationRequest{PerPage: 2},
		}))
		if err != nil {
			t.Fatalf("list after %q: %v", cursor, err)
		}

		for _, entry := range res.Msg.Entries {
			walked = append(walked, entry.Key)
		}

		cursor = res.Msg.NextCursor
		if cursor == "" {
			break
		}
	}

	if want := []string{"long.one", "long.two", "ok.one", "ok.two", "save.case", "save.editing", "save.exact"}; !slices.Equal(walked, want) {
		t.Errorf("same text: got %v, want %v", walked, want)
	}

	list, err := server.ListEntries(ctx, connect.NewRequest(&pb.ListEntriesRequest{
		Duplicates:      pb.DuplicateFilter_DUPLICATE_FILTER_TRANSLATED_DIFFERENTLY,
		StatusLocaleIds: []string{de.ID},
	}))
	if err != nil {
		t.Fatal(err)
	}

	var different []string
	for _, entry := range list.Msg.Entries {
		different = append(different, entry.Key)
	}

	if want := []string{"save.case", "save.editing", "save.exact"}; !slices.Equal(different, want) {
		t.Errorf("translated differently: got %v, want %v", different, want)
	}
}
