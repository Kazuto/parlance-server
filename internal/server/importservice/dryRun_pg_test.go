package importservice

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
	"google.golang.org/protobuf/proto"
	"gorm.io/datatypes"
)

// snapshot returns every row an import writes, deleted ones too, so a dry run can be
// checked to leave them as they were
func snapshot(t *testing.T, db *database.DB) map[string][]map[string]any {
	t.Helper()

	tables := map[string]string{
		"entries":              "id",
		"entry_scopes":         "entry_id, scope_id",
		"localizations":        "id",
		"localization_history": "id",
		"webhook_deliveries":   "id",
		"task_runs":            "id",
	}
	rows := map[string][]map[string]any{}

	for table, order := range tables {
		var found []map[string]any
		if err := db.Table(table).Order(order).Find(&found).Error; err != nil {
			t.Fatal(err)
		}

		rows[table] = found
	}

	return rows
}

// counts lists an import result's counts
func counts(result *pb.ImportResult) []int32 {
	return []int32{
		result.EntriesCreated, result.EntriesUpdated, result.EntriesSkipped, result.EntriesFailed,
		result.TranslationsAdded, result.TranslationsChanged, result.TranslationsRemoved, result.TranslationsOutdated,
	}
}

// keepsNothing checks the rows are as they were before
func keepsNothing(t *testing.T, db *database.DB, name string, before map[string][]map[string]any) {
	t.Helper()

	after := snapshot(t, db)

	for table, rows := range before {
		if !reflect.DeepEqual(rows, after[table]) {
			t.Errorf("%s: %s changed from %v to %v", name, table, rows, after[table])
		}
	}
}

func zipArchive(t *testing.T, files ...[2]string) []byte {
	t.Helper()

	var archive bytes.Buffer

	writer := zip.NewWriter(&archive)

	for _, file := range files {
		w, err := writer.Create(file[0])
		if err != nil {
			t.Fatal(err)
		}

		if _, err := w.Write([]byte(file[1])); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return archive.Bytes()
}

func TestDryRunKeepsNothing(t *testing.T) {
	f := setupOutdatedFixture(t)

	// A webhook and a task that every kind of import would send something to
	hook := models.Webhook{
		Name: "CI", URL: "https://ci.example/hook", Enabled: true,
		Events: []string{
			models.WebhookEventEntriesCreated, models.WebhookEventTranslationsApproved, models.WebhookEventTranslationsAwaitingApproval,
		},
	}
	task := models.Task{
		Name: "Translate", Type: models.TaskTypeAITranslate, Enabled: true,
		Events: []string{models.EventTranslationsApproved},
	}

	for _, row := range []any{&hook, &task} {
		if err := f.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	importJSON := func(req *pb.ImportJsonRequest) *pb.ImportResult {
		t.Helper()

		res, err := f.server.ImportJson(f.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("ImportJson(%v) error = %v", req, err)
		}

		return res.Msg.Result
	}

	// Each import is tried first, then kept, so the next one starts from its changes.
	imports := []struct {
		name string
		req  *pb.ImportJsonRequest
	}{
		{"approving replace-all", &pb.ImportJsonRequest{
			LocaleId: f.german.ID, ScopeIds: []string{f.scope.ID}, Mode: pb.ImportMode_IMPORT_MODE_REPLACE_ALL, Approve: true,
			FileContent: []byte(`{"a":"A-neu","b":"B-de","new":"Neu"}`),
		}},
		{"default text", &pb.ImportJsonRequest{LocaleId: f.english.ID, FileContent: []byte(`{"a":"A2","new":"New"}`)}},
		{"drafts", &pb.ImportJsonRequest{LocaleId: f.german.ID, FileContent: []byte(`{"a":"A2-de","other":"Andere"}`)}},
	}

	for _, item := range imports {
		before := snapshot(t, f.db)

		item.req.DryRun = true
		dry := importJSON(item.req)

		keepsNothing(t, f.db, item.name, before)

		item.req.DryRun = false
		kept := importJSON(item.req)

		if !slices.Equal(counts(dry), counts(kept)) {
			t.Errorf("%s: dry run counts %v, kept import %v", item.name, counts(dry), counts(kept))
		}

		if len(kept.Changes) != 0 {
			t.Errorf("%s: kept import lists %d changes, want none", item.name, len(kept.Changes))
		}
	}

	// The kept imports did send something.
	if after := snapshot(t, f.db); len(after["webhook_deliveries"]) == 0 || len(after["task_runs"]) == 0 {
		t.Errorf("%d webhook deliveries and %d task runs, want some of each", len(after["webhook_deliveries"]), len(after["task_runs"]))
	}
}

func TestDryRunListsChanges(t *testing.T) {
	f := setupOutdatedFixture(t)
	dryRun := func(req *pb.ImportJsonRequest) *pb.ImportResult {
		t.Helper()

		req.DryRun = true

		res, err := f.server.ImportJson(f.ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("ImportJson(%v) error = %v", req, err)
		}

		return res.Msg.Result
	}

	// "b" is imported again as it is, "c" not at all, and "new" is a new entry.
	result := dryRun(&pb.ImportJsonRequest{
		LocaleId: f.german.ID, ScopeIds: []string{f.scope.ID}, Mode: pb.ImportMode_IMPORT_MODE_REPLACE_ALL,
		FileContent: []byte(`{"a":"A-neu","b":"B-de","new":"Neu"}`),
	})
	want := []*pb.ImportChange{
		{Key: "a", Action: pb.ImportChangeAction_IMPORT_CHANGE_ACTION_CHANGE, OldTranslation: "A-de", NewTranslation: "A-neu", SourceTranslation: "A"},
		{Key: "c", Action: pb.ImportChangeAction_IMPORT_CHANGE_ACTION_REMOVE, OldTranslation: "C-de"},
		{Key: "new", Action: pb.ImportChangeAction_IMPORT_CHANGE_ACTION_ADD, NewTranslation: "Neu"},
	}

	checkChanges(t, "replace-all", result.Changes, want)

	// Replace-all creates "a" and "b" again, but only changes "a".
	if got := counts(result); !slices.Equal(got, []int32{3, 0, 0, 0, 1, 1, 1, 0}) {
		t.Errorf("replace-all counts = %v", got)
	}

	// A default text lists no default text of its own, and outdates the German "a".
	result = dryRun(&pb.ImportJsonRequest{LocaleId: f.english.ID, FileContent: []byte(`{"a":"A2","b":"B"}`)})

	checkChanges(t, "default text", result.Changes, []*pb.ImportChange{
		{Key: "a", Action: pb.ImportChangeAction_IMPORT_CHANGE_ACTION_CHANGE, OldTranslation: "A", NewTranslation: "A2"},
	})

	if got := counts(result); !slices.Equal(got, []int32{0, 2, 0, 0, 0, 1, 0, 1}) {
		t.Errorf("default text counts = %v", got)
	}

	// Only the first changes by key are listed, but all are counted.
	many := map[string]string{}
	for i := range maxImportChanges + 1 {
		many[fmt.Sprintf("k%03d", i)] = "Text"
	}

	content, _ := json.Marshal(many)
	result = dryRun(&pb.ImportJsonRequest{LocaleId: f.german.ID, FileContent: content})

	if len(result.Changes) != maxImportChanges || result.TranslationsAdded != maxImportChanges+1 ||
		result.Changes[0].Key != "k000" || result.Changes[maxImportChanges-1].Key != fmt.Sprintf("k%03d", maxImportChanges-1) {
		t.Errorf("listed %d changes from %s to %s of %d added", len(result.Changes),
			result.Changes[0].Key, result.Changes[len(result.Changes)-1].Key, result.TranslationsAdded)
	}
}

func TestZipDryRunImportsFilesInOrder(t *testing.T) {
	f := setupOutdatedFixture(t)

	broken := models.Locale{Code: "xc", Names: datatypes.JSON(`{"en":"Broken"}`)}
	if err := f.db.Create(&broken).Error; err != nil {
		t.Fatal(err)
	}

	// The broken file fails in the database, which must not stop the German one after it.
	archive := zipArchive(t,
		[2]string{"xc.json", `{"a":"bad\u0000"}`},
		[2]string{"xb.json", `{"a":"A2-de","fresh":"Frisch"}`},
		[2]string{"xa.json", `{"a":"A2","fresh":"Fresh"}`},
	)
	importZip := func(dryRun bool) map[string]*pb.LocaleImportResult {
		t.Helper()

		res, err := f.server.ImportZip(f.ctx, connect.NewRequest(&pb.ImportZipRequest{FileContent: archive, DryRun: dryRun}))
		if err != nil {
			t.Fatalf("ImportZip(dry run %v) error = %v", dryRun, err)
		}

		results := map[string]*pb.LocaleImportResult{}
		for _, result := range res.Msg.Results {
			results[result.LocaleCode] = result
		}

		return results
	}

	before := snapshot(t, f.db)
	dry := importZip(true)

	keepsNothing(t, f.db, "zip", before)

	if dry["xc"].Error == "" || dry["xc"].Result != nil {
		t.Errorf("broken file result = %+v, want an error", dry["xc"])
	}

	// The German file sees the English one's new entry and texts.
	if german := dry["xb"].GetResult(); german == nil {
		t.Errorf("German result = %+v", dry["xb"])
	} else {
		checkChanges(t, "German", german.Changes, []*pb.ImportChange{
			{Key: "a", Action: pb.ImportChangeAction_IMPORT_CHANGE_ACTION_CHANGE, OldTranslation: "A-de", NewTranslation: "A2-de", SourceTranslation: "A2"},
			{Key: "fresh", Action: pb.ImportChangeAction_IMPORT_CHANGE_ACTION_ADD, NewTranslation: "Frisch", SourceTranslation: "Fresh"},
		})
	}

	kept := importZip(false)

	for _, code := range []string{"xa", "xb"} {
		if got, want := counts(dry[code].GetResult()), counts(kept[code].GetResult()); !slices.Equal(got, want) {
			t.Errorf("%s: dry run counts %v, kept import %v", code, got, want)
		}
	}

	if kept["xc"].Error == "" {
		t.Errorf("kept broken file result = %+v, want an error", kept["xc"])
	}
}

func checkChanges(t *testing.T, name string, got, want []*pb.ImportChange) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("%s: changes %v, want %v", name, got, want)

		return
	}

	for i := range want {
		if !proto.Equal(got[i], want[i]) {
			t.Errorf("%s: change %d = %v, want %v", name, i, got[i], want[i])
		}
	}
}
