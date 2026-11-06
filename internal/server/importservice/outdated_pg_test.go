package importservice

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

// outdatedFixture has an English default and German translations of "a" and "b", and
// of "c", which has no English text yet. All entries are in one scope.
type outdatedFixture struct {
	db              *database.DB
	ctx             context.Context
	server          *Server
	english, german models.Locale
	scope           models.Scope
	entries         map[string]*models.Entry
}

func setupOutdatedFixture(t *testing.T) *outdatedFixture {
	t.Helper()

	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "create_entry", "update_entry", "delete_entry", "review_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	f := &outdatedFixture{
		db:      db,
		ctx:     ctx,
		server:  &Server{db: db},
		english: models.Locale{Code: "xa", IsDefault: true, Names: datatypes.JSON(`{"en":"Source"}`)},
		german:  models.Locale{Code: "xb", Names: datatypes.JSON(`{"en":"Target"}`)},
		scope:   models.Scope{Name: "app", Slug: "app"},
		entries: map[string]*models.Entry{"a": {Key: "a"}, "b": {Key: "b"}, "c": {Key: "c"}},
	}

	for _, row := range []any{&f.english, &f.german, &f.scope, f.entries["a"], f.entries["b"], f.entries["c"]} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	texts := []struct{ key, locale, text string }{
		{"a", f.english.ID, "A"}, {"b", f.english.ID, "B"},
		{"a", f.german.ID, "A-de"}, {"b", f.german.ID, "B-de"}, {"c", f.german.ID, "C-de"},
	}

	for _, text := range texts {
		entry := f.entries[text.key]
		if err := db.Model(entry).Association("Scopes").Append(&f.scope); err != nil {
			t.Fatal(err)
		}

		if err := db.Create(&models.Localization{EntryID: entry.ID, LocaleID: text.locale, Translation: text.text, Review: models.Review{Status: models.StatusApproved}}).Error; err != nil {
			t.Fatal(err)
		}
	}

	return f
}

// outdated returns the keys whose German translation is outdated
func (f *outdatedFixture) outdated(t *testing.T) map[string]bool {
	t.Helper()

	var keys []string
	if err := f.db.Model(&models.Localization{}).
		Joins("JOIN entries ON entries.id = localizations.entry_id").
		Where("localizations.locale_id = ? AND localizations.outdated_at IS NOT NULL", f.german.ID).
		Pluck("entries.key", &keys).Error; err != nil {
		t.Fatal(err)
	}

	result := map[string]bool{}
	for _, key := range keys {
		result[key] = true
	}

	return result
}

func (f *outdatedFixture) markGerman(t *testing.T, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if err := f.db.Model(&models.Localization{}).
			Where("entry_id = ? AND locale_id = ?", f.entries[key].ID, f.german.ID).
			UpdateColumn("outdated_at", time.Now()).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func (f *outdatedFixture) importJSON(t *testing.T, req *pb.ImportJsonRequest) {
	t.Helper()

	if _, err := f.server.ImportJson(f.ctx, connect.NewRequest(req)); err != nil {
		t.Fatalf("ImportJson() error = %v", err)
	}
}

func checkKeys(t *testing.T, name string, got map[string]bool, want ...string) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("%s: outdated %v, want %v", name, got, want)

		return
	}

	for _, key := range want {
		if !got[key] {
			t.Errorf("%s: outdated %v, want %v", name, got, want)

			return
		}
	}
}

func TestDefaultImportMarksChangedEntries(t *testing.T) {
	f := setupOutdatedFixture(t)

	// "b" stays the same; "c" gets its first English text.
	f.importJSON(t, &pb.ImportJsonRequest{
		LocaleId:    f.english.ID,
		FileContent: []byte(`{"a":"A2","b":"B","c":"C","d":"D"}`),
	})

	checkKeys(t, "after importing English", f.outdated(t), "a", "c")
}

func TestTranslationImportMakesTextCurrent(t *testing.T) {
	f := setupOutdatedFixture(t)
	f.markGerman(t, "a", "b", "c")

	f.importJSON(t, &pb.ImportJsonRequest{
		LocaleId:    f.german.ID,
		FileContent: []byte(`{"a":"A-de2","b":"B-de"}`),
	})
	checkKeys(t, "after changing a", f.outdated(t), "b", "c")

	f.importJSON(t, &pb.ImportJsonRequest{
		LocaleId:    f.german.ID,
		Approve:     true,
		FileContent: []byte(`{"b":"B-de"}`),
	})
	checkKeys(t, "after approving b", f.outdated(t), "c")
}

func TestReplaceAllImportComparesWithTheReplacedText(t *testing.T) {
	f := setupOutdatedFixture(t)

	f.importJSON(t, &pb.ImportJsonRequest{
		LocaleId:    f.english.ID,
		ScopeIds:    []string{f.scope.ID},
		Mode:        pb.ImportMode_IMPORT_MODE_REPLACE_ALL,
		FileContent: []byte(`{"a":"A","b":"B2"}`),
	})
	checkKeys(t, "after replacing English", f.outdated(t), "b")

	// The same German text again stays outdated; c's new one is current.
	f.markGerman(t, "c")
	f.importJSON(t, &pb.ImportJsonRequest{
		LocaleId:    f.german.ID,
		ScopeIds:    []string{f.scope.ID},
		Mode:        pb.ImportMode_IMPORT_MODE_REPLACE_ALL,
		FileContent: []byte(`{"a":"A-de","b":"B-de","c":"C-de2"}`),
	})
	checkKeys(t, "after replacing German", f.outdated(t), "b")
}

func TestZipImportWritesTheDefaultTextFirst(t *testing.T) {
	f := setupOutdatedFixture(t)

	// The German file comes first in the archive and updates "a" for its new English.
	var archive bytes.Buffer

	writer := zip.NewWriter(&archive)
	for _, file := range []struct{ name, content string }{
		{"xb.json", `{"a":"A2-de"}`},
		{"xa.json", `{"a":"A2"}`},
	} {
		w, err := writer.Create(file.name)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := w.Write([]byte(file.content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	res, err := f.server.ImportZip(f.ctx, connect.NewRequest(&pb.ImportZipRequest{FileContent: archive.Bytes()}))
	if err != nil {
		t.Fatalf("ImportZip() error = %v", err)
	}

	for _, result := range res.Msg.Results {
		if result.Error != "" {
			t.Fatalf("importing %s: %s", result.LocaleCode, result.Error)
		}
	}

	checkKeys(t, "after importing both", f.outdated(t))
}
