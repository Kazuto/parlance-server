package importservice

import (
	"context"
	"slices"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

// deleteMissingFixture holds the default language, another one and two scopes
type deleteMissingFixture struct {
	ctx            context.Context
	db             *database.DB
	server         *Server
	english, other models.Locale
	app, web       models.Scope
}

func setupDeleteMissing(t *testing.T) *deleteMissingFixture {
	t.Helper()

	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "create_entry", "update_entry", "delete_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	f := &deleteMissingFixture{
		ctx: ctx, db: db, server: &Server{db: db},
		english: models.Locale{Code: "xa", IsDefault: true, Names: datatypes.JSON(`{"en":"Source"}`)},
		other:   models.Locale{Code: "xb", Names: datatypes.JSON(`{"en":"Target"}`)},
		app:     models.Scope{Name: "app", Slug: "app"},
		web:     models.Scope{Name: "web", Slug: "web"},
	}

	for _, row := range []any{&f.english, &f.other, &f.app, &f.web} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	return f
}

// entry creates an entry with a default text in the scopes
func (f *deleteMissingFixture) entry(t *testing.T, key string, scopes ...models.Scope) models.Entry {
	t.Helper()

	entry := models.Entry{Key: key}
	if err := f.db.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}

	if len(scopes) > 0 {
		if err := f.db.Model(&entry).Association("Scopes").Append(scopes); err != nil {
			t.Fatal(err)
		}
	}

	if err := f.db.Create(&models.Localization{EntryID: entry.ID, LocaleID: f.english.ID, Translation: "Text of " + key}).Error; err != nil {
		t.Fatal(err)
	}

	return entry
}

// liveKeys lists the keys of the entries that aren't deleted
func (f *deleteMissingFixture) liveKeys(t *testing.T) []string {
	t.Helper()

	var keys []string
	if err := f.db.Model(&models.Entry{}).Order("key").Pluck("key", &keys).Error; err != nil {
		t.Fatal(err)
	}

	return keys
}

func (f *deleteMissingFixture) importJSON(localeID string, scopeIDs []string, content string, dryRun bool) (*pb.ImportResult, error) {
	response, err := f.server.ImportJson(f.ctx, connect.NewRequest(&pb.ImportJsonRequest{
		LocaleId: localeID, ScopeIds: scopeIDs, FileContent: []byte(content), DeleteMissing: true, DryRun: dryRun,
	}))
	if err != nil {
		return nil, err
	}

	return response.Msg.Result, nil
}

func TestImportDeletesTheEntriesTheFileDoesntHold(t *testing.T) {
	f := setupDeleteMissing(t)
	f.entry(t, "kept")
	stale := f.entry(t, "stale")

	preview, err := f.importJSON(f.english.ID, nil, `{"kept":"Kept","new":"New"}`, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}

	want := []*pb.ImportDeletedEntry{{Key: "stale", SourceTranslation: "Text of stale"}}
	if deletion := preview.Deletion; deletion.EntriesDeleted != 1 || deletion.EntriesUnscoped != 0 ||
		!slices.EqualFunc(deletion.Entries, want, func(a, b *pb.ImportDeletedEntry) bool {
			return a.Key == b.Key && a.SourceTranslation == b.SourceTranslation && a.Unscoped == b.Unscoped
		}) {
		t.Fatalf("dry run deletion = %v, want stale listed", deletion)
	}

	if keys := f.liveKeys(t); !slices.Equal(keys, []string{"kept", "stale"}) {
		t.Fatalf("a dry run left %v", keys)
	}

	result, err := f.importJSON(f.english.ID, nil, `{"kept":"Kept","new":"New"}`, false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if result.Deletion.EntriesDeleted != 1 || len(result.Deletion.Entries) != 0 {
		t.Errorf("deletion = %v, want one entry, not listed", result.Deletion)
	}

	if keys := f.liveKeys(t); !slices.Equal(keys, []string{"kept", "new"}) {
		t.Errorf("entries %v, want kept and new", keys)
	}

	var deleted models.Entry
	if err := f.db.Unscoped().First(&deleted, "id = ?", stale.ID).Error; err != nil {
		t.Fatal(err)
	}

	if userID := middleware.UserID(f.ctx); !deleted.DeletedAt.Valid || deleted.UpdatedBy == nil || *deleted.UpdatedBy != userID {
		t.Errorf("stale entry %+v, want it deleted by %s", deleted, userID)
	}
}

func TestImportDeletesMissingEntriesOfItsScopesOnly(t *testing.T) {
	f := setupDeleteMissing(t)
	f.entry(t, "app.kept", f.app)
	f.entry(t, "app.stale", f.app)
	shared := f.entry(t, "shared", f.app, f.web)
	f.entry(t, "web.only", f.web)
	f.entry(t, "unscoped")

	// The shared entry's tags: the app one leaves with the app scope.
	appTag := models.Tag{ScopeID: f.app.ID, Name: "checkout"}
	webTag := models.Tag{ScopeID: f.web.ID, Name: "header"}

	for _, row := range []any{&appTag, &webTag} {
		if err := f.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	for _, tag := range []models.Tag{appTag, webTag} {
		if err := f.db.Create(&models.EntryTag{EntryID: shared.ID, TagID: tag.ID, ScopeID: tag.ScopeID}).Error; err != nil {
			t.Fatal(err)
		}
	}

	preview, err := f.importJSON(f.english.ID, []string{f.app.ID}, `{"app.kept":"Kept"}`, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}

	if got := preview.Deletion.Entries; len(got) != 2 || got[0].Key != "app.stale" || got[0].Unscoped ||
		got[1].Key != "shared" || !got[1].Unscoped {
		t.Fatalf("dry run lists %v, want app.stale deleted and shared unscoped", got)
	}

	result, err := f.importJSON(f.english.ID, []string{f.app.ID}, `{"app.kept":"Kept"}`, false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if result.Deletion.EntriesDeleted != 1 || result.Deletion.EntriesUnscoped != 1 {
		t.Errorf("deletion = %v, want one deleted and one unscoped", result.Deletion)
	}

	if keys := f.liveKeys(t); !slices.Equal(keys, []string{"app.kept", "shared", "unscoped", "web.only"}) {
		t.Errorf("entries %v, want only app.stale deleted", keys)
	}

	var scopes []string
	if err := f.db.Model(&models.EntryScope{}).Where("entry_id = ?", shared.ID).Pluck("scope_id", &scopes).Error; err != nil {
		t.Fatal(err)
	}

	var tags []string
	if err := f.db.Model(&models.EntryTag{}).Where("entry_id = ?", shared.ID).Pluck("tag_id", &tags).Error; err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(scopes, []string{f.web.ID}) || !slices.Equal(tags, []string{webTag.ID}) {
		t.Errorf("shared entry has scopes %v and tags %v, want only web's", scopes, tags)
	}
}

func TestImportDeleteMissingRejectsFilesThatDontHoldAllEntries(t *testing.T) {
	f := setupDeleteMissing(t)
	f.entry(t, "kept")

	// Another language's file may leave out the entries it doesn't translate, and an
	// empty file would delete them all.
	for _, request := range []struct{ localeID, content string }{
		{f.other.ID, `{"new":"Neu"}`},
		{f.english.ID, `{}`},
	} {
		if _, err := f.importJSON(request.localeID, nil, request.content, false); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: error = %v, want invalid argument", request.content, err)
		}
	}

	if keys := f.liveKeys(t); !slices.Equal(keys, []string{"kept"}) {
		t.Errorf("entries %v, want nothing imported or deleted", keys)
	}
}

func TestPoImportDeleteMissingKeepsTheRecordsItDoesntImport(t *testing.T) {
	f := setupDeleteMissing(t)

	for _, key := range []string{"empty", "plural", "stale"} {
		f.entry(t, key)
	}

	po := "msgid \"kept\"\nmsgstr \"Kept\"\n\n" +
		"msgid \"empty\"\nmsgstr \"\"\n\n" +
		"msgid \"plural\"\nmsgid_plural \"plurals\"\nmsgstr[0] \"One\"\nmsgstr[1] \"Many\"\n"

	_, err := f.server.ImportPo(f.ctx, connect.NewRequest(&pb.ImportPoRequest{
		LocaleId: f.english.ID, FileContent: []byte(po), DeleteMissing: true,
	}))
	if err != nil {
		t.Fatalf("ImportPo() error = %v", err)
	}

	if keys := f.liveKeys(t); !slices.Equal(keys, []string{"empty", "kept", "plural"}) {
		t.Errorf("entries %v, want only stale deleted", keys)
	}
}

func TestZipImportDeletesTheEntriesTheDefaultFileDoesntHold(t *testing.T) {
	f := setupDeleteMissing(t)
	f.entry(t, "kept")
	f.entry(t, "stale")

	importZip := func(dryRun bool, files ...[2]string) (*pb.ImportZipResponse, error) {
		response, err := f.server.ImportZip(f.ctx, connect.NewRequest(&pb.ImportZipRequest{
			FileContent: zipArchive(t, files...), DeleteMissing: true, DryRun: dryRun,
		}))
		if err != nil {
			return nil, err
		}

		return response.Msg, nil
	}

	if _, err := importZip(false, [2]string{"xb.json", `{"kept":"Behalten"}`}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("without the default language's file: error = %v, want invalid argument", err)
	}

	// The other language holds a key the default one doesn't: it is skipped, not
	// created and then deleted.
	files := [][2]string{{"xb.json", `{"kept":"Behalten","old":"Alt"}`}, {"app/xa.json", `{"kept":"Kept"}`}, {"web/xa.json", `{"new":"New"}`}}

	preview, err := importZip(true, files...)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}

	if got := preview.Deletion.Entries; len(got) != 1 || got[0].Key != "stale" {
		t.Errorf("dry run lists %v, want stale", got)
	}

	response, err := importZip(false, files...)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if response.Deletion.EntriesDeleted != 1 {
		t.Errorf("deletion = %v, want one entry", response.Deletion)
	}

	for _, result := range response.Results {
		if result.LocaleCode == "xb" && result.Result.EntriesSkipped != 1 {
			t.Errorf("xb.json: %v, want old skipped", result.Result)
		}
	}

	var keys []string
	if err := f.db.Unscoped().Model(&models.Entry{}).Order("key").Pluck("key", &keys).Error; err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(keys, []string{"kept", "new", "stale"}) || !slices.Equal(f.liveKeys(t), []string{"kept", "new"}) {
		t.Errorf("entries %v, live %v; want stale deleted and old never created", keys, f.liveKeys(t))
	}
}
