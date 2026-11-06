package locale

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

func TestDestroyLocaleRemovesTranslatedLocale(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "permanent_delete_locale")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	locale := models.Locale{Code: "xx", Names: datatypes.JSON(`{"en":"Test"}`)}
	entry := models.Entry{Key: "greeting"}

	if err := db.Create(&locale).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&entry).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&models.Localization{EntryID: entry.ID, LocaleID: locale.ID, Translation: "Hallo"}).Error; err != nil {
		t.Fatal(err)
	}

	term := models.Terminology{Term: "greeting"}
	if err := db.Create(&term).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&models.Definition{TerminologyID: term.ID, LocaleID: locale.ID, Translation: "Gruß"}).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Delete(&locale).Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{db: db}
	if _, err := server.DestroyLocale(ctx, connect.NewRequest(&pb.DestroyLocaleRequest{Id: locale.ID})); err != nil {
		t.Fatalf("DestroyLocale() error = %v", err)
	}

	var remaining int64

	db.Unscoped().Model(&models.Locale{}).Where("id = ?", locale.ID).Count(&remaining)

	if remaining != 0 {
		t.Errorf("locale still exists")
	}

	db.Unscoped().Model(&models.Localization{}).Where("locale_id = ?", locale.ID).Count(&remaining)

	if remaining != 0 {
		t.Errorf("%d localizations still exist", remaining)
	}

	db.Unscoped().Model(&models.Definition{}).Where("locale_id = ?", locale.ID).Count(&remaining)

	if remaining != 0 {
		t.Errorf("%d definitions still exist", remaining)
	}
}
