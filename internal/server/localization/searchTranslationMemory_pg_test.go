package localization

import (
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

func TestSearchTranslationMemory(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "read_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	en := models.Locale{Code: "xa", IsDefault: true, Names: datatypes.JSON(`{"en":"Source"}`)}
	de := models.Locale{Code: "xb", Names: datatypes.JSON(`{"en":"Target"}`)}
	fr := models.Locale{Code: "xc", Names: datatypes.JSON(`{"en":"Other"}`)}

	for _, locale := range []*models.Locale{&en, &de, &fr} {
		if err := db.Create(locale).Error; err != nil {
			t.Fatal(err)
		}
	}

	approved := models.Review{Status: models.StatusApproved}
	entries := map[string]string{}
	add := func(key, source, german string, review models.Review, outdated bool) {
		t.Helper()

		entry := models.Entry{Key: key}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}

		entries[key] = entry.ID
		rows := []models.Localization{
			{EntryID: entry.ID, LocaleID: en.ID, Translation: source, Review: approved},
			{EntryID: entry.ID, LocaleID: de.ID, Translation: german, Review: review},
		}

		if outdated {
			rows[1].OutdatedAt = new(time.Now())
		}

		if err := db.Create(&rows).Error; err != nil {
			t.Fatal(err)
		}
	}

	// Similarity to "Save your changes": 1, 0.81, 0.76 and 0.71. The trigram index
	// also picks the next two, which are too different: 0.59, and 0.30 for the same
	// words in another order. It doesn't pick the last.
	add("save.exact", "Save your changes", "Änderungen speichern", approved, false)
	add("save.exact.copy", "Save your changes", "Änderungen speichern", approved, false)
	add("save.now", "Save your changes now", "Änderungen jetzt speichern", approved, false)
	add("save.all", "Save all changes", "Alle speichern", approved, false)
	add("save.short", "Save changes", "Speichern", approved, false)
	add("save.work", "Save your work", "Arbeit speichern", approved, false)
	add("save.done", "Your changes were saved", "Deine Änderungen wurden gespeichert", approved, false)
	add("account.delete", "Delete your account", "Konto löschen", approved, false)
	// Left out: a draft, an outdated translation, a deleted entry and the edited one.
	add("draft", "Save your changes", "Entwurf", models.Review{Status: models.StatusDraft}, false)
	add("outdated", "Save your changes", "Veraltet", approved, true)
	add("deleted", "Save your changes", "Gelöscht", approved, false)
	add("editing", "Save your changes", "In Bearbeitung", approved, false)
	// Short texts are alike by chance, so even an identical one is no match.
	add("ok", "OK", "OK", approved, false)

	if err := db.Delete(&models.Entry{}, "id = ?", entries["deleted"]).Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{db: db}
	search := func(req *pb.SearchTranslationMemoryRequest) []*pb.TranslationMemoryMatch {
		t.Helper()

		res, err := server.SearchTranslationMemory(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("SearchTranslationMemory(%v) error = %v", req, err)
		}

		return res.Msg.Matches
	}
	keys := func(matches []*pb.TranslationMemoryMatch) []string {
		result := make([]string, len(matches))
		for i, match := range matches {
			result[i] = match.Key
		}

		return result
	}
	check := func(name string, got []*pb.TranslationMemoryMatch, want ...string) {
		t.Helper()

		if got := keys(got); !slices.Equal(got, want) {
			t.Errorf("%s: matches %v, want %v", name, got, want)
		}
	}

	matches := search(&pb.SearchTranslationMemoryRequest{
		SourceText:     "save your changes",
		TargetLocaleId: de.ID,
		ExcludeEntryId: entries["editing"],
	})
	check("best first", matches, "save.exact", "save.now", "save.all", "save.short")

	if best := matches[0]; best.SourceText != "Save your changes" || best.Translation != "Änderungen speichern" ||
		best.EntryId != entries["save.exact"] || best.Similarity != 1 {
		t.Errorf("best match = %+v", best)
	}

	if similarity := matches[3].Similarity; similarity < 0.7 || similarity > 0.71 {
		t.Errorf("similarity of save.short = %v, want about 0.706", similarity)
	}

	check("limited", search(&pb.SearchTranslationMemoryRequest{
		SourceText: "Save your changes", TargetLocaleId: de.ID, ExcludeEntryId: entries["editing"], Limit: 2,
	}), "save.exact", "save.now")

	// Without leaving it out, the edited entry is one of the identical texts.
	check("not excluding", search(&pb.SearchTranslationMemoryRequest{
		SourceText: "Save your changes", TargetLocaleId: de.ID, Limit: 2,
	}), "editing", "save.exact")

	check("no translations", search(&pb.SearchTranslationMemoryRequest{SourceText: "Save your changes", TargetLocaleId: fr.ID}))
	check("too short", search(&pb.SearchTranslationMemoryRequest{SourceText: " OK ", TargetLocaleId: de.ID}))

	testutil.ExpectInvalidArgument(t, ctx, server.SearchTranslationMemory,
		&pb.SearchTranslationMemoryRequest{SourceText: "Save your changes"},
		&pb.SearchTranslationMemoryRequest{SourceText: "Save your changes", TargetLocaleId: en.ID},
	)
}
