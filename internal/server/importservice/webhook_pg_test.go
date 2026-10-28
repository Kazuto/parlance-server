package importservice

import (
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/webhook"
	"gorm.io/datatypes"
)

func TestImportsAreSentToWebhooks(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "import_entry", "review_entry")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	locale := models.Locale{Code: "xx", Names: datatypes.JSON(`{"en":"Test"}`)}
	existing := models.Entry{Key: "existing"}
	hook := models.Webhook{
		Name: "CI", URL: "https://ci.example/hook", Enabled: true,
		Events: []string{
			models.WebhookEventEntriesCreated, models.WebhookEventTranslationsApproved, models.WebhookEventTranslationsAwaitingApproval,
		},
	}

	for _, row := range []any{&locale, &existing, &hook} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	server := &Server{db: db}
	importJSON := func(translations map[string]string, approve bool) {
		t.Helper()

		content, _ := json.Marshal(translations)
		if _, err := server.ImportJson(ctx, connect.NewRequest(&pb.ImportJsonRequest{LocaleId: locale.ID, FileContent: content, Approve: approve})); err != nil {
			t.Fatalf("import: %v", err)
		}
	}
	sent := func(event string, data any) int {
		t.Helper()

		var deliveries []models.WebhookDelivery

		db.Where("webhook_id = ? AND event = ?", hook.ID, event).Order("created_at DESC").Find(&deliveries)

		if len(deliveries) > 0 {
			body := struct {
				Data any `json:"data"`
			}{Data: data}
			if err := json.Unmarshal([]byte(deliveries[0].Payload), &body); err != nil {
				t.Fatalf("payload: %v", err)
			}
		}

		return len(deliveries)
	}

	// A plain import sends its new entries, and its translations as awaiting approval.
	importJSON(map[string]string{"existing": "Alt", "added": "Neu"}, false)

	var drafted webhook.TranslationsData
	if count := sent(models.WebhookEventTranslationsAwaitingApproval, &drafted); count != 1 || drafted.Count != 2 {
		t.Errorf("%d translations.awaiting_approval deliveries, newest %+v", count, drafted)
	}

	var entries webhook.EntriesData
	if count := sent(models.WebhookEventEntriesCreated, &entries); count != 1 || entries.Count != 1 || entries.Entries[0].Key != "added" || entries.User == nil {
		t.Errorf("%d entries.created deliveries, newest %+v", count, entries)
	}

	if count := sent(models.WebhookEventTranslationsApproved, nil); count != 0 {
		t.Errorf("%d translations.approved deliveries after a plain import, want none", count)
	}

	// An approving import sends what it approved; it creates no entries this time.
	importJSON(map[string]string{"existing": "Alt", "added": "Neu"}, true)

	var translations webhook.TranslationsData
	if count := sent(models.WebhookEventTranslationsApproved, &translations); count != 1 || translations.Count != 2 ||
		translations.Translations[0].Key != "added" || translations.Translations[0].Locale != "xx" {
		t.Errorf("%d translations.approved deliveries, newest %+v", count, translations)
	}

	if count := sent(models.WebhookEventEntriesCreated, nil); count != 1 {
		t.Errorf("%d entries.created deliveries, want still 1", count)
	}

	// Only changed texts await approval again; unchanged ones keep their review.
	importJSON(map[string]string{"existing": "Alt", "added": "Neuer"}, false)

	if count := sent(models.WebhookEventTranslationsAwaitingApproval, &drafted); count != 2 || drafted.Count != 1 ||
		drafted.Translations[0].Key != "added" || drafted.Translations[0].Translation != "Neuer" {
		t.Errorf("%d translations.awaiting_approval deliveries, newest %+v", count, drafted)
	}
}
