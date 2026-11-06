package webhookservice

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/webhook"
)

func setup(t *testing.T) (context.Context, *Server) {
	t.Helper()

	ctx, db := testutil.Setup(t)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM webhook_deliveries`)
		db.Exec(`DELETE FROM webhooks`)
	})

	box, _ := secret.New("test-secret")

	return ctx, NewServer(db, box, webhook.NewSender(box)).(*Server)
}

// receiver answers with the status and records the signatures it was sent.
func receiver(t *testing.T, status int) (*httptest.Server, func() []string) {
	t.Helper()

	var (
		mu         sync.Mutex
		signatures []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		signatures = append(signatures, r.Header.Get(webhook.HeaderSignature))
		mu.Unlock()

		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)

	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()

		return append([]string{}, signatures...)
	}
}

var approved = []pb.WebhookEvent{pb.WebhookEvent_WEBHOOK_EVENT_TRANSLATIONS_APPROVED}

func TestCreateWebhook(t *testing.T) {
	ctx, server := setup(t)

	t.Run("keeps the secret encrypted and to itself", func(t *testing.T) {
		res, err := server.CreateWebhook(ctx, connect.NewRequest(&pb.CreateWebhookRequest{
			Name:    " CI ",
			Url:     " https://ci.example/hooks/parlance ",
			Events:  append(approved, approved[0], pb.WebhookEvent_WEBHOOK_EVENT_TASK_RUN_FAILED),
			Enabled: true,
			Secret:  " s3cret ",
		}))
		if err != nil {
			t.Fatalf("CreateWebhook() error = %v", err)
		}

		hook := res.Msg.Webhook
		if hook.Name != "CI" || hook.Url != "https://ci.example/hooks/parlance" || len(hook.Events) != 2 || !hook.Enabled || !hook.HasSecret {
			t.Errorf("webhook = %+v", hook)
		}

		var stored models.Webhook

		server.db.First(&stored, "id = ?", hook.Id)

		if plain, err := server.box.Decrypt(stored.Secret); err != nil || plain != "s3cret" {
			t.Errorf("stored secret = %q, %v", plain, err)
		}
	})

	t.Run("rejects invalid settings", func(t *testing.T) {
		testutil.ExpectInvalidArgument(t, ctx, server.CreateWebhook,
			&pb.CreateWebhookRequest{Name: " ", Url: "https://ci.example", Events: approved},
			&pb.CreateWebhookRequest{Name: "No URL", Events: approved},
			&pb.CreateWebhookRequest{Name: "FTP", Url: "ftp://ci.example", Events: approved},
			&pb.CreateWebhookRequest{Name: "Relative", Url: "/hooks", Events: approved},
			&pb.CreateWebhookRequest{Name: "No events", Url: "https://ci.example"},
			&pb.CreateWebhookRequest{Name: "Ping", Url: "https://ci.example", Events: []pb.WebhookEvent{pb.WebhookEvent_WEBHOOK_EVENT_PING}},
			&pb.CreateWebhookRequest{Name: "Unknown", Url: "https://ci.example", Events: []pb.WebhookEvent{pb.WebhookEvent_WEBHOOK_EVENT_UNSPECIFIED}},
		)
	})

	t.Run("posts JSON unless told otherwise", func(t *testing.T) {
		create := func(format pb.WebhookFormat) pb.WebhookFormat {
			res, err := server.CreateWebhook(ctx, connect.NewRequest(&pb.CreateWebhookRequest{
				Name: "Chat", Url: "https://discord.example/api/webhooks/1/abc", Events: approved, Format: format,
			}))
			if err != nil {
				t.Fatalf("CreateWebhook() error = %v", err)
			}

			return res.Msg.Webhook.Format
		}

		if got := create(pb.WebhookFormat_WEBHOOK_FORMAT_UNSPECIFIED); got != pb.WebhookFormat_WEBHOOK_FORMAT_JSON {
			t.Errorf("unspecified format = %v, want JSON", got)
		}

		if got := create(pb.WebhookFormat_WEBHOOK_FORMAT_DISCORD); got != pb.WebhookFormat_WEBHOOK_FORMAT_DISCORD {
			t.Errorf("format = %v, want Discord", got)
		}

		testutil.ExpectInvalidArgument(t, ctx, server.CreateWebhook,
			&pb.CreateWebhookRequest{Name: "Odd", Url: "https://ci.example", Events: approved, Format: pb.WebhookFormat(99)},
		)
	})

	t.Run("needs an encryption key for a secret", func(t *testing.T) {
		withoutKey := &Server{db: server.db, box: &secret.Box{}, sender: server.sender}

		_, err := withoutKey.CreateWebhook(ctx, connect.NewRequest(&pb.CreateWebhookRequest{
			Name: "CI", Url: "https://ci.example", Events: approved, Secret: "s3cret",
		}))
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("error = %v, want failed precondition", err)
		}
	})
}

func TestUpdateWebhook(t *testing.T) {
	ctx, server := setup(t)

	created, err := server.CreateWebhook(ctx, connect.NewRequest(&pb.CreateWebhookRequest{
		Name: "CI", Url: "https://ci.example", Events: approved, Enabled: true, Secret: "first",
	}))
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}

	id := created.Msg.Webhook.Id
	update := func(t *testing.T, req *pb.UpdateWebhookRequest) *pb.Webhook {
		t.Helper()

		req.Id = id

		res, err := server.UpdateWebhook(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateWebhook() error = %v", err)
		}

		return res.Msg.Webhook
	}
	stored := func() models.Webhook {
		var hook models.Webhook

		server.db.First(&hook, "id = ?", id)

		return hook
	}

	t.Run("changes only what is sent", func(t *testing.T) {
		before := stored()
		name := "Builds"

		hook := update(t, &pb.UpdateWebhookRequest{
			Name:   &name,
			Events: &pb.WebhookEventSelection{Events: []pb.WebhookEvent{pb.WebhookEvent_WEBHOOK_EVENT_ENTRIES_CREATED}},
		})
		if hook.Name != "Builds" || hook.Url != "https://ci.example" || !hook.Enabled || !hook.HasSecret ||
			len(hook.Events) != 1 || hook.Events[0] != pb.WebhookEvent_WEBHOOK_EVENT_ENTRIES_CREATED || stored().Secret != before.Secret {
			t.Errorf("webhook = %+v", hook)
		}
	})

	t.Run("removes the secret when sent empty", func(t *testing.T) {
		empty := ""

		if hook := update(t, &pb.UpdateWebhookRequest{Secret: &empty}); hook.HasSecret || stored().Secret != "" {
			t.Errorf("webhook = %+v, want no secret", hook)
		}
	})

	t.Run("turning it off fails its pending deliveries", func(t *testing.T) {
		now := time.Now()
		pending := models.WebhookDelivery{
			WebhookID: id, EventID: "event", Event: models.WebhookEventTranslationsApproved, Payload: "{}",
			Status: models.DeliveryPending, NextAttemptAt: &now,
		}
		server.db.Create(&pending)

		off := false

		if hook := update(t, &pb.UpdateWebhookRequest{Enabled: &off}); hook.Enabled {
			t.Errorf("webhook = %+v, want it off", hook)
		}

		var failed models.WebhookDelivery

		server.db.First(&failed, "id = ?", pending.ID)

		if failed.Status != models.DeliveryFailed || failed.NextAttemptAt != nil || failed.Error == "" {
			t.Errorf("pending delivery = %+v, want it failed", failed)
		}
	})

	t.Run("rejects invalid settings", func(t *testing.T) {
		blank, ftp := " ", "ftp://ci.example"

		testutil.ExpectInvalidArgument(t, ctx, server.UpdateWebhook,
			&pb.UpdateWebhookRequest{Id: id, Name: &blank},
			&pb.UpdateWebhookRequest{Id: id, Url: &ftp},
			&pb.UpdateWebhookRequest{Id: id, Events: &pb.WebhookEventSelection{}},
		)
	})
}

func TestDeleteWebhook(t *testing.T) {
	ctx, server := setup(t)

	hook := models.Webhook{Name: "CI", URL: "https://ci.example", Events: []string{models.WebhookEventEntriesCreated}}
	server.db.Create(&hook)
	server.db.Create(&models.WebhookDelivery{WebhookID: hook.ID, EventID: "event", Event: models.WebhookEventEntriesCreated, Payload: "{}", Status: models.DeliverySucceeded})

	if _, err := server.DeleteWebhook(ctx, connect.NewRequest(&pb.DeleteWebhookRequest{Id: hook.ID})); err != nil {
		t.Fatalf("DeleteWebhook() error = %v", err)
	}

	var left int64

	server.db.Model(&models.WebhookDelivery{}).Where("webhook_id = ?", hook.ID).Count(&left)

	if left != 0 {
		t.Errorf("%d deliveries left", left)
	}

	if _, err := server.DeleteWebhook(ctx, connect.NewRequest(&pb.DeleteWebhookRequest{Id: hook.ID})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("deleting again: error = %v, want not found", err)
	}
}

func TestTestAndRedeliver(t *testing.T) {
	ctx, server := setup(t)
	receiving, signatures := receiver(t, http.StatusAccepted)

	created, err := server.CreateWebhook(ctx, connect.NewRequest(&pb.CreateWebhookRequest{
		Name: "CI", Url: receiving.URL, Events: approved, Secret: "s3cret",
	}))
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}

	id := created.Msg.Webhook.Id

	// Webhooks that are off can be tested too.
	tested, err := server.TestWebhook(ctx, connect.NewRequest(&pb.TestWebhookRequest{Id: id}))
	if err != nil {
		t.Fatalf("TestWebhook() error = %v", err)
	}

	ping := tested.Msg.Delivery
	if ping.Event != pb.WebhookEvent_WEBHOOK_EVENT_PING || ping.Status != pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_SUCCEEDED ||
		ping.ResponseStatus != http.StatusAccepted || ping.Attempts != 1 || ping.Payload == "" {
		t.Fatalf("ping = %+v", ping)
	}

	var body struct {
		Event string `json:"event"`
		Data  struct {
			Webhook struct {
				ID     string   `json:"id"`
				Events []string `json:"events"`
			} `json:"webhook"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(ping.Payload), &body); err != nil || body.Event != "ping" || body.Data.Webhook.ID != id ||
		len(body.Data.Webhook.Events) != 1 || body.Data.Webhook.Events[0] != models.WebhookEventTranslationsApproved {
		t.Errorf("ping body = %s (%v)", ping.Payload, err)
	}

	if got := signatures(); len(got) != 1 || got[0] != webhook.Sign("s3cret", []byte(ping.Payload)) {
		t.Errorf("signatures = %v", got)
	}

	redelivered, err := server.RedeliverWebhookDelivery(ctx, connect.NewRequest(&pb.RedeliverWebhookDeliveryRequest{Id: ping.Id}))
	if err != nil {
		t.Fatalf("RedeliverWebhookDelivery() error = %v", err)
	}

	again := redelivered.Msg.Delivery
	if again.Id == ping.Id || again.Payload != ping.Payload || again.Status != pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_SUCCEEDED {
		t.Errorf("redelivery = %+v", again)
	}

	listed, err := server.ListWebhookDeliveries(ctx, connect.NewRequest(&pb.ListWebhookDeliveriesRequest{WebhookId: id}))
	if err != nil {
		t.Fatalf("ListWebhookDeliveries() error = %v", err)
	}

	deliveries := listed.Msg.Deliveries
	if len(deliveries) != 2 || listed.Msg.Pagination.Total != 2 || deliveries[0].Payload != "" || deliveries[0].ResponseStatus != http.StatusAccepted {
		t.Errorf("deliveries = %+v, want both without payloads", deliveries)
	}

	got, err := server.GetWebhookDelivery(ctx, connect.NewRequest(&pb.GetWebhookDeliveryRequest{Id: ping.Id}))
	if err != nil || got.Msg.Delivery.Payload != ping.Payload {
		t.Errorf("GetWebhookDelivery() = %+v, %v", got, err)
	}

	list, err := server.ListWebhooks(ctx, connect.NewRequest(&pb.ListWebhooksRequest{}))
	if err != nil {
		t.Fatalf("ListWebhooks() error = %v", err)
	}

	if hooks := list.Msg.Webhooks; len(hooks) != 1 || hooks[0].LastDelivery == nil || hooks[0].LastDelivery.Payload != "" {
		t.Errorf("webhooks = %+v, want the newest delivery without its payload", hooks)
	}

	if _, err := server.GetWebhookDelivery(ctx, connect.NewRequest(&pb.GetWebhookDeliveryRequest{Id: "missing"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("missing delivery: error = %v, want not found", err)
	}
}

func TestFormatChangesApplyToRedeliveries(t *testing.T) {
	ctx, server := setup(t)

	var (
		mu     sync.Mutex
		bodies []string
	)

	receiving := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()

		// Like Discord, which turns down anything but its own messages.
		if !strings.Contains(string(body), `"content"`) {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer receiving.Close()

	created, err := server.CreateWebhook(ctx, connect.NewRequest(&pb.CreateWebhookRequest{Name: "Discord", Url: receiving.URL, Events: approved}))
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}

	id := created.Msg.Webhook.Id

	tested, err := server.TestWebhook(ctx, connect.NewRequest(&pb.TestWebhookRequest{Id: id}))
	if err != nil || tested.Msg.Delivery.ResponseStatus != http.StatusBadRequest {
		t.Fatalf("JSON ping = %+v, %v; want it turned down", tested, err)
	}

	discord := pb.WebhookFormat_WEBHOOK_FORMAT_DISCORD
	if _, err := server.UpdateWebhook(ctx, connect.NewRequest(&pb.UpdateWebhookRequest{Id: id, Format: &discord})); err != nil {
		t.Fatalf("UpdateWebhook() error = %v", err)
	}

	// The failed ping now shows, and is sent again, as a Discord message.
	got, err := server.GetWebhookDelivery(ctx, connect.NewRequest(&pb.GetWebhookDeliveryRequest{Id: tested.Msg.Delivery.Id}))
	if err != nil || !strings.Contains(got.Msg.Delivery.Payload, `"content"`) {
		t.Errorf("delivery body = %q, %v", got.Msg.Delivery.GetPayload(), err)
	}

	again, err := server.RedeliverWebhookDelivery(ctx, connect.NewRequest(&pb.RedeliverWebhookDeliveryRequest{Id: tested.Msg.Delivery.Id}))
	if err != nil || again.Msg.Delivery.Status != pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_SUCCEEDED {
		t.Errorf("redelivery = %+v, %v", again, err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(bodies) != 2 || strings.Contains(bodies[0], `"content"`) || !strings.Contains(bodies[1], "Parlance webhook") {
		t.Errorf("bodies = %q", bodies)
	}
}

func TestFailedTestIsReported(t *testing.T) {
	ctx, server := setup(t)
	receiving, _ := receiver(t, http.StatusNotFound)

	hook := models.Webhook{Name: "CI", URL: receiving.URL, Events: []string{models.WebhookEventEntriesCreated}, Enabled: true}
	server.db.Create(&hook)

	res, err := server.TestWebhook(ctx, connect.NewRequest(&pb.TestWebhookRequest{Id: hook.ID}))
	if err != nil {
		t.Fatalf("TestWebhook() error = %v", err)
	}

	if delivery := res.Msg.Delivery; delivery.Status != pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_FAILED ||
		delivery.ResponseStatus != http.StatusNotFound || delivery.Error == "" || delivery.NextAttemptAt != nil {
		t.Errorf("delivery = %+v, want a failure that isn't tried again", delivery)
	}
}

func TestDigestSchedules(t *testing.T) {
	ctx, server := setup(t)

	created, err := server.CreateWebhook(ctx, connect.NewRequest(&pb.CreateWebhookRequest{
		Name: "Daily", Url: "https://chat.example/hook", Events: approved, Enabled: true, Schedule: " 0 9 * * * ",
	}))
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}

	hook := created.Msg.Webhook
	if hook.Schedule != "0 9 * * *" || hook.NextDigestAt == nil {
		t.Fatalf("webhook = %+v, want a daily digest planned", hook)
	}

	testutil.ExpectInvalidArgument(t, ctx, server.CreateWebhook,
		&pb.CreateWebhookRequest{Name: "Bad cron", Url: "https://chat.example", Events: approved, Schedule: "daily"},
		&pb.CreateWebhookRequest{Name: "Digest", Url: "https://chat.example", Events: []pb.WebhookEvent{pb.WebhookEvent_WEBHOOK_EVENT_DIGEST}},
	)

	// Two events wait for the digest; a test went out meanwhile.
	for range 2 {
		server.db.Create(&models.WebhookDelivery{
			WebhookID: hook.Id, EventID: "event", Event: models.WebhookEventTranslationsApproved, Payload: "{}", Status: models.DeliveryCollected,
		})
	}

	sent := models.WebhookDelivery{
		WebhookID: hook.Id, EventID: "ping", Event: models.WebhookEventPing, Payload: "{}", Status: models.DeliverySucceeded,
		CreatedAt: time.Now().Add(-time.Hour),
	}
	server.db.Create(&sent)

	list, err := server.ListWebhooks(ctx, connect.NewRequest(&pb.ListWebhooksRequest{}))
	if err != nil {
		t.Fatalf("ListWebhooks() error = %v", err)
	}

	if listed := list.Msg.Webhooks[0]; listed.CollectedCount != 2 || listed.LastDelivery == nil || listed.LastDelivery.Id != sent.ID {
		t.Errorf("webhook = %+v, want 2 collected and the ping as last delivery", listed)
	}

	// Without a schedule, what it collected goes out right away.
	none := ""

	res, err := server.UpdateWebhook(ctx, connect.NewRequest(&pb.UpdateWebhookRequest{Id: hook.Id, Schedule: &none}))
	if err != nil {
		t.Fatalf("UpdateWebhook() error = %v", err)
	}

	if next := res.Msg.Webhook.NextDigestAt; res.Msg.Webhook.Schedule != "" || next == nil {
		t.Errorf("webhook = %+v, want a digest right away", res.Msg.Webhook)
	} else if at, _ := time.Parse(time.RFC3339, *next); time.Since(at) > time.Minute {
		t.Errorf("next digest at %s, want now", *next)
	}

	// Turned off, it drops what it collected and plans nothing.
	off := false

	res, err = server.UpdateWebhook(ctx, connect.NewRequest(&pb.UpdateWebhookRequest{Id: hook.Id, Enabled: &off}))
	if err != nil {
		t.Fatalf("UpdateWebhook() error = %v", err)
	}

	if res.Msg.Webhook.NextDigestAt != nil || res.Msg.Webhook.CollectedCount != 0 {
		t.Errorf("webhook = %+v, want nothing collected or planned", res.Msg.Webhook)
	}
}
