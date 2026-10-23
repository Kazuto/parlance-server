package entry

import (
	"sort"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

func TestListEntriesFiltersByScopeAndStatus(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "read_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	locale := models.Locale{Code: "xx", Names: datatypes.JSON(`{"en":"Test"}`)}
	app, web := models.Scope{Name: "App", Slug: "app"}, models.Scope{Name: "Web", Slug: "web"}
	shared, appOnly, unscoped := models.Entry{Key: "shared"}, models.Entry{Key: "app.only"}, models.Entry{Key: "unscoped"}

	for _, row := range []any{&locale, &app, &web, &shared, &appOnly, &unscoped} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	for entry, scopes := range map[*models.Entry][]models.Scope{&shared: {app, web}, &appOnly: {app}} {
		if err := db.Model(entry).Association("Scopes").Append(scopes); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Create(&models.Localization{EntryID: shared.ID, LocaleID: locale.ID, Translation: "Geteilt"}).Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{db: db}
	list := func(req *pb.ListEntriesRequest) []string {
		t.Helper()

		res, err := server.ListEntries(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("ListEntries(%v) error = %v", req, err)
		}

		var keys []string
		for _, entry := range res.Msg.Entries {
			keys = append(keys, entry.Key)
		}

		sort.Strings(keys)

		return keys
	}
	check := func(name string, got []string, want ...string) {
		t.Helper()

		if len(got) != len(want) {
			t.Errorf("%s: got %v, want %v", name, got, want)

			return
		}

		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: got %v, want %v", name, got, want)

				return
			}
		}
	}

	// An entry in both scopes is listed once.
	check("both scopes", list(&pb.ListEntriesRequest{ScopeIds: []string{app.ID, web.ID}}), "app.only", "shared")
	check("web scope", list(&pb.ListEntriesRequest{ScopeIds: []string{web.ID}}), "shared")
	check("missing", list(&pb.ListEntriesRequest{TranslationStatus: "missing", TargetLocaleIds: []string{locale.ID}}), "app.only", "unscoped")
	check("complete", list(&pb.ListEntriesRequest{TranslationStatus: "complete", TargetLocaleIds: []string{locale.ID}}), "shared")
}

func TestListEntriesFindsDraftsForReview(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "read_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	de := models.Locale{Code: "de", Names: datatypes.JSON(`{"en":"German"}`)}
	fr := models.Locale{Code: "fr", Names: datatypes.JSON(`{"en":"French"}`)}
	approved, draft := models.Entry{Key: "approved"}, models.Entry{Key: "draft"}

	for _, row := range []any{&de, &fr, &approved, &draft} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	for _, localization := range []models.Localization{
		{EntryID: approved.ID, LocaleID: de.ID, Translation: "Ja", Review: models.Review{Status: models.StatusApproved}},
		{EntryID: draft.ID, LocaleID: de.ID, Translation: "Ja", Review: models.Review{Status: models.StatusApproved}},
		{EntryID: draft.ID, LocaleID: fr.ID, Translation: "Oui", Review: models.Review{Status: models.StatusDraft}},
	} {
		if err := db.Create(&localization).Error; err != nil {
			t.Fatal(err)
		}
	}

	server := &Server{db: db}
	list := func(localeIDs ...string) *pb.ListEntriesResponse {
		t.Helper()

		res, err := server.ListEntries(ctx, connect.NewRequest(&pb.ListEntriesRequest{
			Summary: true, TranslationStatus: "review", TargetLocaleIds: localeIDs,
		}))
		if err != nil {
			t.Fatalf("ListEntries() error = %v", err)
		}

		return res.Msg
	}

	if res := list(de.ID, fr.ID); len(res.Entries) != 1 || res.Entries[0].Key != "draft" ||
		len(res.Entries[0].DraftLocaleIds) != 1 || res.Entries[0].DraftLocaleIds[0] != fr.ID {
		t.Errorf("review in de and fr = %+v, want draft with its French draft", res.Entries)
	}

	// The draft is French, so it is out of sight when only German is shown.
	if res := list(de.ID); len(res.Entries) != 0 {
		t.Errorf("review in de = %+v, want none", res.Entries)
	}
}
