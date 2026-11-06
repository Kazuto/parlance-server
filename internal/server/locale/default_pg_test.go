package locale

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

func TestOneDefaultLocaleAtATime(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	server := &Server{db: db}

	create := func(code string, isDefault bool) string {
		t.Helper()

		res, err := server.CreateLocale(ctx, connect.NewRequest(&pb.CreateLocaleRequest{
			Code:      code,
			IsDefault: isDefault,
			Names:     &pb.StringMap{Values: map[string]string{"en": code}},
		}))
		if err != nil {
			t.Fatalf("CreateLocale(%s) error = %v", code, err)
		}

		return res.Msg.Locale.Id
	}
	defaults := func() []string {
		t.Helper()

		var codes []string

		db.Model(&models.Locale{}).Where("is_default = ?", true).Pluck("code", &codes)

		return codes
	}

	// Bootstrap may already have made a default; each step must leave exactly one.
	create("aa", true)

	if got := defaults(); len(got) != 1 || got[0] != "aa" {
		t.Fatalf("after creating a default: defaults = %v, want [aa]", got)
	}

	bb := create("bb", false)
	if _, err := server.SetDefaultLocale(ctx, connect.NewRequest(&pb.SetDefaultLocaleRequest{LocaleId: bb})); err != nil {
		t.Fatalf("SetDefaultLocale() error = %v", err)
	}

	if got := defaults(); len(got) != 1 || got[0] != "bb" {
		t.Fatalf("after setting a default: defaults = %v, want [bb]", got)
	}
	// Saving the current default again keeps it.
	isDefault := true
	if _, err := server.UpdateLocale(ctx, connect.NewRequest(&pb.UpdateLocaleRequest{Id: bb, IsDefault: &isDefault})); err != nil {
		t.Fatalf("UpdateLocale() error = %v", err)
	}

	if got := defaults(); len(got) != 1 || got[0] != "bb" {
		t.Fatalf("after re-saving the default: defaults = %v, want [bb]", got)
	}
}

// Translations are compared with the default text, so a new default starts afresh.
func TestNewDefaultLocaleClearsOutdatedTranslations(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	en := models.Locale{Code: "xa", IsDefault: true, Names: datatypes.JSON(`{"en":"Source"}`)}
	de := models.Locale{Code: "xb", Names: datatypes.JSON(`{"en":"Target"}`)}
	entry := models.Entry{Key: "greeting"}

	for _, row := range []any{&en, &de, &entry} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	german := models.Localization{EntryID: entry.ID, LocaleID: de.ID, Translation: "Hallo", OutdatedAt: new(time.Now())}
	if err := db.Create(&german).Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{db: db}
	outdated := func() bool {
		t.Helper()

		var localization models.Localization
		if err := db.First(&localization, "id = ?", german.ID).Error; err != nil {
			t.Fatal(err)
		}

		return localization.OutdatedAt != nil
	}

	// Saving the default as it is keeps the marks.
	isDefault := true
	if _, err := server.UpdateLocale(ctx, connect.NewRequest(&pb.UpdateLocaleRequest{Id: en.ID, IsDefault: &isDefault})); err != nil {
		t.Fatalf("UpdateLocale() error = %v", err)
	}

	if !outdated() {
		t.Fatal("re-saving the default cleared the mark")
	}

	if _, err := server.SetDefaultLocale(ctx, connect.NewRequest(&pb.SetDefaultLocaleRequest{LocaleId: de.ID})); err != nil {
		t.Fatalf("SetDefaultLocale() error = %v", err)
	}

	if outdated() {
		t.Error("the new default left the mark")
	}
}
