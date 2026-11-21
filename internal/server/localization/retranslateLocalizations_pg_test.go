package localization

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestRetranslateLocalizations(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "update_entry", "use_machine_translation")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	en := models.Locale{Code: "en", IsDefault: true}
	de := models.Locale{Code: "de"}
	fr := models.Locale{Code: "fr"}

	for _, locale := range []*models.Locale{&en, &de, &fr} {
		if err := db.Create(locale).Error; err != nil {
			t.Fatal(err)
		}
	}

	web := models.Scope{Name: "Web", Slug: "web"}
	app := models.Scope{Name: "App", Slug: "app"}

	for _, scope := range []*models.Scope{&web, &app} {
		if err := db.Create(scope).Error; err != nil {
			t.Fatal(err)
		}
	}

	// German texts by entry key, written by a model unless a person's
	ids := map[string]string{}
	add := func(key string, scope models.Scope, person bool) models.Entry {
		t.Helper()

		entry := models.Entry{Key: key, Scopes: []models.Scope{scope}}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}

		for _, locale := range []models.Locale{de, fr} {
			text := models.Localization{EntryID: entry.ID, LocaleID: locale.ID, Translation: key, Review: models.Review{Status: models.StatusApproved}}
			if !person {
				text.WrittenBy("old-model")
			}

			if err := db.Create(&text).Error; err != nil {
				t.Fatal(err)
			}

			if locale.ID == de.ID {
				ids[key] = text.ID
			}
		}

		return entry
	}

	add("web.machine", web, false)
	webPerson := add("web.person", web, true)
	appMachine := add("app.machine", app, false)

	if err := db.Delete(ptr(add("web.deleted", web, false))).Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{db: db}
	call := func(req *pb.RetranslateLocalizationsRequest) (count, pending int32) {
		t.Helper()

		res, err := server.RetranslateLocalizations(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("RetranslateLocalizations(%v) error = %v", req, err)
		}

		return res.Msg.Count, res.Msg.Pending
	}
	marked := func() map[string]bool {
		t.Helper()

		var texts []models.Localization

		db.Unscoped().Where("retranslate_at IS NOT NULL").Find(&texts)

		keys := map[string]bool{}

		for _, text := range texts {
			for key, id := range ids {
				if id == text.ID {
					keys[key] = true
				}
			}

			if text.LocaleID != de.ID {
				t.Errorf("a %s text is marked", text.LocaleID)
			}
		}

		return keys
	}

	testutil.ExpectInvalidArgument(t, ctx, server.RetranslateLocalizations,
		&pb.RetranslateLocalizationsRequest{},
		// The default language is the one translated from.
		&pb.RetranslateLocalizationsRequest{LocaleId: en.ID},
	)

	// Dry runs count, and mark nothing.
	if count, pending := call(&pb.RetranslateLocalizationsRequest{LocaleId: de.ID, DryRun: true}); count != 2 || pending != 0 {
		t.Errorf("dry run: %d, %d pending; want the 2 live machine texts", count, pending)
	}

	if count, _ := call(&pb.RetranslateLocalizationsRequest{LocaleId: de.ID, ScopeIds: []string{web.ID}, IncludeHuman: true, DryRun: true}); count != 2 {
		t.Errorf("dry run of web with people's texts: %d, want 2", count)
	}

	if got := marked(); len(got) != 0 {
		t.Fatalf("dry runs marked %v", got)
	}

	var before models.Localization

	db.First(&before, "id = ?", ids["web.machine"])

	time.Sleep(10 * time.Millisecond)

	if count, pending := call(&pb.RetranslateLocalizationsRequest{LocaleId: de.ID, ScopeIds: []string{web.ID}}); count != 1 || pending != 1 {
		t.Errorf("marking web: %d, %d pending; want web's machine text", count, pending)
	}

	// Marking doesn't change the text, so it keeps its update time and history.
	var after models.Localization

	db.First(&after, "id = ?", ids["web.machine"])

	if !after.UpdatedAt.Equal(before.UpdatedAt) || after.RetranslateAt == nil || after.Status != models.StatusApproved {
		t.Errorf("marked text: %+v, want it marked and otherwise unchanged", after)
	}

	// Marked ones count once.
	if count, pending := call(&pb.RetranslateLocalizationsRequest{LocaleId: de.ID, IncludeHuman: true}); count != 2 || pending != 3 {
		t.Errorf("marking all: %d, %d pending; want the other 2 of 3", count, pending)
	}

	// Taking the marks back goes by scope, whoever wrote the texts.
	if count, pending := call(&pb.RetranslateLocalizationsRequest{LocaleId: de.ID, ScopeIds: []string{app.ID}, Cancel: true}); count != 1 || pending != 0 {
		t.Errorf("taking app's back: %d, %d pending in app; want 1, 0", count, pending)
	}

	if got := marked(); len(got) != 2 || !got["web.machine"] || !got["web.person"] {
		t.Errorf("marked: %v, want web's texts", got)
	}

	// A person's new text clears the mark.
	res, err := server.UpdateLocalization(ctx, connect.NewRequest(&pb.UpdateLocalizationRequest{
		Id: ids["web.person"], Translation: ptr("Neu"),
	}))
	if err != nil {
		t.Fatalf("UpdateLocalization() error = %v", err)
	}

	if res.Msg.Localization.Retranslate {
		t.Error("the edited text is still marked")
	}

	if got := marked(); len(got) != 1 || !got["web.machine"] {
		t.Errorf("marked after the edit: %v, want web.machine", got)
	}

	// Ticked entries: only theirs, and with scopes only those in both.
	ticked := []string{webPerson.ID, appMachine.ID}

	if count, pending := call(&pb.RetranslateLocalizationsRequest{LocaleId: de.ID, EntryIds: ticked, IncludeHuman: true, DryRun: true}); count != 2 || pending != 0 {
		t.Errorf("dry run of the ticked entries: %d, %d pending; want both, none pending", count, pending)
	}

	if count, _ := call(&pb.RetranslateLocalizationsRequest{LocaleId: de.ID, EntryIds: ticked, ScopeIds: []string{app.ID}, IncludeHuman: true, DryRun: true}); count != 1 {
		t.Errorf("dry run of the ticked entries in app: %d, want 1", count)
	}

	if count, pending := call(&pb.RetranslateLocalizationsRequest{LocaleId: de.ID, EntryIds: ticked, IncludeHuman: true}); count != 2 || pending != 2 {
		t.Errorf("marking the ticked entries: %d, %d pending; want 2, 2", count, pending)
	}

	if got := marked(); len(got) != 3 {
		t.Errorf("marked: %v, want web.machine and the ticked entries' texts", got)
	}

	// Taking back the ticked entries' marks leaves the others'.
	if count, pending := call(&pb.RetranslateLocalizationsRequest{LocaleId: de.ID, EntryIds: ticked, Cancel: true}); count != 2 || pending != 0 {
		t.Errorf("taking the ticked entries' back: %d, %d pending; want 2, 0", count, pending)
	}

	if got := marked(); len(got) != 1 || !got["web.machine"] {
		t.Errorf("marked after taking back: %v, want web.machine", got)
	}
}

func ptr[T any](value T) *T {
	return &value
}
