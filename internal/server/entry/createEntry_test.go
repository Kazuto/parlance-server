package entry

import (
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/webhook"
)

func TestCreatedEntriesAreSentToWebhooks(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}

	hook := models.Webhook{Name: "Chat", URL: "https://chat.example/hook", Enabled: true, Events: []string{models.WebhookEventEntriesCreated}}
	db.Create(&hook)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM webhook_deliveries WHERE webhook_id = ?`, hook.ID)
		db.Exec(`DELETE FROM webhooks WHERE id = ?`, hook.ID)
		db.Exec(`DELETE FROM entries WHERE key = 'webhook.created'`)
	})

	res, err := server.CreateEntry(ctx, connect.NewRequest(&pb.CreateEntryRequest{Key: "webhook.created", Description: "Sent along"}))
	if err != nil {
		t.Fatalf("CreateEntry() error = %v", err)
	}

	var deliveries []models.WebhookDelivery

	db.Where("webhook_id = ?", hook.ID).Find(&deliveries)

	if len(deliveries) != 1 || deliveries[0].Event != models.WebhookEventEntriesCreated {
		t.Fatalf("deliveries = %+v, want one entries.created", deliveries)
	}

	var body struct {
		Data webhook.EntriesData `json:"data"`
	}

	if err := json.Unmarshal([]byte(deliveries[0].Payload), &body); err != nil {
		t.Fatalf("payload: %v", err)
	}

	want := webhook.Entry{ID: res.Msg.Entry.Id, Key: "webhook.created", Description: "Sent along"}
	if body.Data.Count != 1 || body.Data.Entries[0] != want || body.Data.User == nil || body.Data.User.ID != middleware.UserID(ctx) {
		t.Errorf("data = %+v", body.Data)
	}
}
