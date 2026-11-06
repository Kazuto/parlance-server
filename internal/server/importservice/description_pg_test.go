package importservice

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

func TestPoImportSetsDescriptions(t *testing.T) {
	f := setupOutdatedFixture(t)

	if err := f.db.Model(f.entries["b"]).Update("description", "Kept").Error; err != nil {
		t.Fatal(err)
	}

	importPo := func(req *pb.ImportPoRequest) {
		t.Helper()

		if _, err := f.server.ImportPo(f.ctx, connect.NewRequest(req)); err != nil {
			t.Fatalf("ImportPo() error = %v", err)
		}
	}

	descriptions := func() map[string]string {
		t.Helper()

		var entries []models.Entry
		if err := f.db.Find(&entries).Error; err != nil {
			t.Fatal(err)
		}

		result := map[string]string{}
		for _, entry := range entries {
			result[entry.Key] = entry.Description
		}

		return result
	}

	// "a" gets one, "b" has none in the file, "c" is untranslated and "new" is created.
	content := []byte(`#. Save button
#. in the toolbar
msgid "a"
msgstr "A"

msgid "b"
msgstr "B"

#. Not imported
msgid "c"
msgstr ""

#. Created with it
msgid "new"
msgstr "New"
`)

	importPo(&pb.ImportPoRequest{LocaleId: f.english.ID, FileContent: content, DryRun: true})

	if got := descriptions(); got["a"] != "" || got["new"] != "" {
		t.Errorf("dry run kept descriptions: %v", got)
	}

	importPo(&pb.ImportPoRequest{LocaleId: f.english.ID, FileContent: content})

	want := map[string]string{"a": "Save button\nin the toolbar", "b": "Kept", "c": "", "new": "Created with it"}
	got := descriptions()

	for key, description := range want {
		if got[key] != description {
			t.Errorf("%s: description = %q, want %q", key, got[key], description)
		}
	}

	// Create-only leaves existing entries, their descriptions too
	importPo(&pb.ImportPoRequest{
		LocaleId: f.german.ID, Mode: pb.ImportMode_IMPORT_MODE_CREATE_ONLY,
		FileContent: []byte("#. Changed\nmsgid \"a\"\nmsgstr \"A-neu\"\n"),
	})

	if got := descriptions(); got["a"] != want["a"] {
		t.Errorf("create-only changed the description to %q", got["a"])
	}
}
