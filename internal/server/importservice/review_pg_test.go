package importservice

import (
	"context"
	"encoding/json"
	"maps"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

func TestImportReviewStates(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	importer, err := testutil.ContextWithPermissions(db, "import_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	reviewer, err := testutil.ContextWithPermissions(db, "import_entry", "review_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	locale := models.Locale{Code: "xx", Names: datatypes.JSON(`{"en":"Test"}`)}
	kept, changed := models.Entry{Key: "kept"}, models.Entry{Key: "changed"}

	for _, row := range []any{&locale, &kept, &changed} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	for _, entry := range []models.Entry{kept, changed} {
		if err := db.Create(&models.Localization{
			EntryID: entry.ID, LocaleID: locale.ID, Translation: "old",
			Review: models.Review{Status: models.StatusApproved},
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	server := &Server{db: db}
	importJSON := func(ctx context.Context, translations map[string]string, approve bool) error {
		content, _ := json.Marshal(translations)
		_, err := server.ImportJson(ctx, connect.NewRequest(&pb.ImportJsonRequest{LocaleId: locale.ID, FileContent: content, Approve: approve}))

		return err
	}
	statuses := func() map[string]string {
		var rows []struct{ Key, Status string }

		db.Table("localizations").Select("entries.key, localizations.status").
			Joins("JOIN entries ON entries.id = localizations.entry_id").
			Where("localizations.locale_id = ?", locale.ID).Scan(&rows)

		result := map[string]string{}
		for _, row := range rows {
			result[row.Key] = row.Status
		}

		return result
	}

	if err := importJSON(importer, map[string]string{"kept": "old", "changed": "new", "added": "neu"}, false); err != nil {
		t.Fatalf("import: %v", err)
	}

	want := map[string]string{"kept": models.StatusApproved, "changed": models.StatusDraft, "added": models.StatusDraft}
	if got := statuses(); !maps.Equal(got, want) {
		t.Errorf("after a plain import: %v, want %v", got, want)
	}

	if err := importJSON(importer, map[string]string{"kept": "old"}, true); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("approving without review_entry: got %v, want permission denied", err)
	}

	if err := importJSON(reviewer, map[string]string{"changed": "new", "added": "neu"}, true); err != nil {
		t.Fatalf("approving import: %v", err)
	}

	want = map[string]string{"kept": models.StatusApproved, "changed": models.StatusApproved, "added": models.StatusApproved}
	if got := statuses(); !maps.Equal(got, want) {
		t.Errorf("after an approving import: %v, want %v", got, want)
	}
}
