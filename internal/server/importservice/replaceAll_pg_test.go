package importservice

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

func TestReplaceAllImportDeletesScopeTranslationsWithHistory(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "create_entry", "update_entry", "delete_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	userID := ctx.Value(middleware.ContextKeyUserID).(string)

	locale := models.Locale{Code: "xx", Names: datatypes.JSON(`{"en":"Test"}`)}
	scope := models.Scope{Name: "app", Slug: "app"}
	kept := models.Entry{Key: "kept"}
	dropped := models.Entry{Key: "dropped"}

	for _, row := range []any{&locale, &scope, &kept, &dropped} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	for _, entry := range []*models.Entry{&kept, &dropped} {
		if err := db.Model(entry).Association("Scopes").Append(&scope); err != nil {
			t.Fatal(err)
		}

		if err := db.Create(&models.Localization{EntryID: entry.ID, LocaleID: locale.ID, Translation: "old"}).Error; err != nil {
			t.Fatal(err)
		}
	}

	server := &Server{db: db}

	_, err = server.ImportJson(ctx, connect.NewRequest(&pb.ImportJsonRequest{
		LocaleId:    locale.ID,
		ScopeIds:    []string{scope.ID},
		Mode:        pb.ImportMode_IMPORT_MODE_REPLACE_ALL,
		FileContent: []byte(`{"kept":"new"}`),
	}))
	if err != nil {
		t.Fatalf("ImportJson() error = %v", err)
	}

	var translations []models.Localization
	if err := db.Where("locale_id = ?", locale.ID).Find(&translations).Error; err != nil {
		t.Fatal(err)
	}

	if len(translations) != 1 || translations[0].EntryID != kept.ID || translations[0].Translation != "new" {
		t.Fatalf("translations after import = %+v, want only kept=new", translations)
	}

	var history models.LocalizationHistory
	if err := db.Where("entry_id = ? AND action = ?", dropped.ID, "deleted").First(&history).Error; err != nil {
		t.Fatalf("no deletion history for dropped entry: %v", err)
	}

	if history.UserID == nil || *history.UserID != userID {
		t.Errorf("deletion credited to %v, want %s", history.UserID, userID)
	}
}

func TestImportIntoUnknownLocaleIsNotFound(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "create_entry", "update_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	server := &Server{db: db}

	_, err = server.ImportJson(ctx, connect.NewRequest(&pb.ImportJsonRequest{
		LocaleId:    "00000000-0000-0000-0000-000000000000",
		FileContent: []byte(`{"key":"value"}`),
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
