package localization

import (
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/webhook"
	"google.golang.org/protobuf/proto"
)

func TestDraftsAreSentToWebhooks(t *testing.T) {
	ctx, db := testutil.Setup(t)
	seedTranslationData(t, db)
	testutil.Exec(t, db, `INSERT INTO entries (id, key) VALUES ('st-extra', 'dashboard.close')`)
	testutil.Exec(t, db, `INSERT INTO localizations (id, entry_id, locale_id, translation) VALUES ('st-l9', 'st-extra', 'st-en', 'Close')`)

	hook := models.Webhook{
		Name: "Reviewers", URL: "https://chat.example/hook", Enabled: true,
		Events: []string{models.WebhookEventTranslationsAwaitingApproval},
	}
	db.Create(&hook)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM webhook_deliveries WHERE webhook_id = ?`, hook.ID)
		db.Exec(`DELETE FROM webhooks WHERE id = ?`, hook.ID)
	})

	server := &Server{db: db, translators: fixedSource{&fakeTranslator{answers: map[string]string{"French": "Ouvrir"}}}}
	sent := func() []webhook.TranslationsData {
		t.Helper()

		var deliveries []models.WebhookDelivery

		db.Where("webhook_id = ?", hook.ID).Order("created_at").Find(&deliveries)

		result := make([]webhook.TranslationsData, len(deliveries))

		for i, delivery := range deliveries {
			var body struct {
				Data webhook.TranslationsData `json:"data"`
			}

			if err := json.Unmarshal([]byte(delivery.Payload), &body); err != nil {
				t.Fatalf("payload: %v", err)
			}

			result[i] = body.Data
		}

		return result
	}

	created, err := server.CreateLocalization(ctx, connect.NewRequest(&pb.CreateLocalizationRequest{
		EntryId: "st-entry", LocaleId: "st-es", Translation: "Abrir",
	}))
	if err != nil {
		t.Fatalf("CreateLocalization() error = %v", err)
	}

	id := created.Msg.Localization.Id

	// Saving the same text again submits nothing; new text does.
	for _, text := range []string{"Abrir", "Abrir el panel"} {
		if _, err := server.UpdateLocalization(ctx, connect.NewRequest(&pb.UpdateLocalizationRequest{Id: id, Translation: proto.String(text)})); err != nil {
			t.Fatalf("UpdateLocalization(%q) error = %v", text, err)
		}
	}

	// A batch of machine translations submits its drafts together.
	if _, err := server.BatchTranslate(ctx, connect.NewRequest(&pb.BatchTranslateRequest{
		EntryIds: []string{"st-entry", "st-extra"}, TargetLocaleId: "st-fr", SourceLocaleId: "st-en",
	})); err != nil {
		t.Fatalf("BatchTranslate() error = %v", err)
	}

	got := sent()
	if len(got) != 3 {
		t.Fatalf("sent %d events, want 3: the new text, the changed text and the batch", len(got))
	}

	if got[0].Count != 1 || got[0].Translations[0].Translation != "Abrir" || got[1].Translations[0].Translation != "Abrir el panel" {
		t.Errorf("editor events = %+v", got[:2])
	}

	if got[2].Count != 2 || got[2].Translations[0].Key != "dashboard.close" || got[2].Translations[1].Locale != "fr" {
		t.Errorf("batch event = %+v", got[2])
	}

	for _, data := range got {
		if data.User == nil || data.User.ID != middleware.UserID(ctx) {
			t.Errorf("event by %+v, want the signed-in user", data.User)
		}
	}
}
