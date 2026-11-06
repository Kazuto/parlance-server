package entry

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/localization"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
	"gorm.io/datatypes"
)

func TestListEntriesFindsOutdatedTranslations(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "read_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	de := models.Locale{Code: "de", Names: datatypes.JSON(`{"en":"German"}`)}
	fr := models.Locale{Code: "fr", Names: datatypes.JSON(`{"en":"French"}`)}
	current, outdated := models.Entry{Key: "current"}, models.Entry{Key: "outdated"}

	for _, row := range []any{&de, &fr, &current, &outdated} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	approved := models.Review{Status: models.StatusApproved}
	for _, localization := range []models.Localization{
		{EntryID: current.ID, LocaleID: de.ID, Translation: "Ja", Review: approved},
		{EntryID: outdated.ID, LocaleID: de.ID, Translation: "Ja", Review: approved},
		{EntryID: outdated.ID, LocaleID: fr.ID, Translation: "Oui", Review: approved},
	} {
		if err := db.Create(&localization).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Model(&models.Localization{}).Where("entry_id = ? AND locale_id = ?", outdated.ID, de.ID).
		UpdateColumn("outdated_at", "2026-01-01T00:00:00Z").Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{db: db}
	list := func(status string, localeIDs ...string) []*pb.Entry {
		t.Helper()

		res, err := server.ListEntries(ctx, connect.NewRequest(&pb.ListEntriesRequest{
			Summary: true, TranslationStatus: status, TargetLocaleIds: localeIDs,
		}))
		if err != nil {
			t.Fatalf("ListEntries() error = %v", err)
		}

		return res.Msg.Entries
	}

	// An approved text can be outdated; it is no draft.
	if entries := list("outdated", de.ID, fr.ID); len(entries) != 1 || entries[0].Key != "outdated" ||
		len(entries[0].OutdatedLocaleIds) != 1 || entries[0].OutdatedLocaleIds[0] != de.ID ||
		len(entries[0].DraftLocaleIds) != 0 {
		t.Errorf("outdated in de and fr = %+v, want outdated with its German", entries)
	}

	// The outdated text is German, so it is out of sight when only French is shown.
	if entries := list("outdated", fr.ID); len(entries) != 0 {
		t.Errorf("outdated in fr = %+v, want none", entries)
	}

	for _, entry := range list("", de.ID, fr.ID) {
		if entry.Key == "current" && len(entry.OutdatedLocaleIds) != 0 {
			t.Errorf("current entry = %+v, want nothing outdated", entry)
		}
	}
}

// The default text a translation was based on is the last one before it was marked.
func TestEntryHistoryBeforeTheDefaultTextChanged(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "read_entry", "update_entry")
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

	source := models.Localization{EntryID: entry.ID, LocaleID: en.ID, Translation: "Hello"}
	german := models.Localization{EntryID: entry.ID, LocaleID: de.ID, Translation: "Hallo"}

	for _, row := range []*models.Localization{&source, &german} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	localizations := localization.NewServer(db, nil, nil)
	for _, text := range []string{"Hi", "Hey"} {
		if _, err := localizations.UpdateLocalization(ctx, connect.NewRequest(&pb.UpdateLocalizationRequest{
			Id: source.ID, Translation: proto.String(text),
		})); err != nil {
			t.Fatalf("UpdateLocalization(%s) error = %v", text, err)
		}
	}

	res, err := localizations.GetLocalization(ctx, connect.NewRequest(&pb.GetLocalizationRequest{Id: german.ID}))
	if err != nil {
		t.Fatalf("GetLocalization() error = %v", err)
	}

	if !res.Msg.Localization.Outdated || res.Msg.Localization.OutdatedAt == "" {
		t.Fatalf("German = %+v, want it outdated", res.Msg.Localization)
	}

	server := &Server{db: db}

	history, err := server.GetEntryHistory(ctx, connect.NewRequest(&pb.GetEntryHistoryRequest{
		EntryId:    entry.ID,
		LocaleId:   en.ID,
		Before:     res.Msg.Localization.OutdatedAt,
		Pagination: &pb.PaginationRequest{Page: 1, PerPage: 1},
	}))
	if err != nil {
		t.Fatalf("GetEntryHistory() error = %v", err)
	}

	if len(history.Msg.History) != 1 || history.Msg.History[0].Translation != "Hello" {
		t.Errorf("default text before = %+v, want Hello", history.Msg.History)
	}

	_, err = server.GetEntryHistory(ctx, connect.NewRequest(&pb.GetEntryHistoryRequest{EntryId: entry.ID, Before: "yesterday"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("GetEntryHistory(before: yesterday) error = %v, want invalid argument", err)
	}
}
