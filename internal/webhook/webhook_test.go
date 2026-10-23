package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// forEachDB runs the test on SQLite and, when configured, on Postgres.
func forEachDB(t *testing.T, test func(t *testing.T, db *gorm.DB)) {
	t.Run("sqlite", func(t *testing.T) {
		test(t, sqliteDB(t))
	})
	t.Run("postgres", func(t *testing.T) {
		test(t, testutil.SetupPostgresDB(t).DB)
	})
}

// sqliteDB is the shared test database, emptied of webhooks when the test ends.
func sqliteDB(t *testing.T) *gorm.DB {
	t.Helper()

	_, db := testutil.Setup(t)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM webhook_deliveries`)
		db.Exec(`DELETE FROM webhooks`)
		db.Exec(`DELETE FROM localizations`)
		db.Exec(`DELETE FROM entries`)
		db.Exec(`DELETE FROM locales`)
	})

	return db.DB
}

func testBox(t *testing.T) *secret.Box {
	t.Helper()

	box, err := secret.New("test key")
	if err != nil {
		t.Fatalf("secret.New() error = %v", err)
	}

	return box
}

func create(t *testing.T, db *gorm.DB, value any) {
	t.Helper()

	if err := db.Create(value).Error; err != nil {
		t.Fatalf("create %T: %v", value, err)
	}
}

func newHook(t *testing.T, db *gorm.DB, url string, events ...string) models.Webhook {
	t.Helper()

	hook := models.Webhook{Name: "Hook", URL: url, Events: events, Enabled: true}
	create(t, db, &hook)

	return hook
}

func deliveriesOf(t *testing.T, db *gorm.DB, webhookID string) []models.WebhookDelivery {
	t.Helper()

	var deliveries []models.WebhookDelivery
	if err := db.Where("webhook_id = ?", webhookID).Order("created_at").Find(&deliveries).Error; err != nil {
		t.Fatalf("load deliveries: %v", err)
	}

	return deliveries
}

func emit(t *testing.T, db *gorm.DB, event string, data any) {
	t.Helper()

	if err := Emit(db, event, func() (any, error) { return data, nil }); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}
}

// received is a request a test receiver got.
type received struct {
	header http.Header
	body   []byte
}

// receiver answers every delivery with the status and records the requests.
func receiver(t *testing.T, status int) (*httptest.Server, func() []received) {
	t.Helper()

	var (
		mu       sync.Mutex
		requests []received
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		mu.Lock()
		requests = append(requests, received{header: r.Header.Clone(), body: body})
		mu.Unlock()

		w.WriteHeader(status)
		fmt.Fprint(w, "thanks")
	}))
	t.Cleanup(server.Close)

	return server, func() []received {
		mu.Lock()
		defer mu.Unlock()

		return append([]received{}, requests...)
	}
}

func TestEmitQueuesSubscribedWebhooks(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *gorm.DB) {
		subscribed := newHook(t, db, "http://example.test/a", models.WebhookEventTranslationsApproved, models.WebhookEventEntriesCreated)
		disabled := newHook(t, db, "http://example.test/b", models.WebhookEventTranslationsApproved)
		db.Model(&disabled).Update("enabled", false)
		other := newHook(t, db, "http://example.test/c", models.WebhookEventTaskRunFailed)

		emit(t, db, models.WebhookEventTranslationsApproved, map[string]int{"count": 2})

		deliveries := deliveriesOf(t, db, subscribed.ID)
		if len(deliveries) != 1 || deliveries[0].Status != models.DeliveryPending || deliveries[0].NextAttemptAt == nil {
			t.Fatalf("deliveries = %+v, want one pending", deliveries)
		}

		var body struct {
			ID        string         `json:"id"`
			Event     string         `json:"event"`
			CreatedAt time.Time      `json:"created_at"`
			Data      map[string]int `json:"data"`
		}

		if err := json.Unmarshal([]byte(deliveries[0].Payload), &body); err != nil {
			t.Fatalf("payload %q: %v", deliveries[0].Payload, err)
		}

		if body.ID != deliveries[0].EventID || body.Event != models.WebhookEventTranslationsApproved || body.CreatedAt.IsZero() || body.Data["count"] != 2 {
			t.Errorf("body = %+v", body)
		}

		if len(deliveriesOf(t, db, disabled.ID)) != 0 || len(deliveriesOf(t, db, other.ID)) != 0 {
			t.Error("a disabled or unsubscribed webhook got a delivery")
		}

		built := false
		if err := Emit(db, models.WebhookEventPing, func() (any, error) {
			built = true

			return nil, nil
		}); err != nil || built {
			t.Errorf("an event nobody wants built its data (err = %v)", err)
		}
	})
}

func TestEventData(t *testing.T) {
	db := sqliteDB(t)

	user := models.User{Email: "reviewer@example.com", Name: "Rita Reviewer", PasswordHash: "unused"}
	locale := models.Locale{Code: "de", Names: datatypes.JSON(`{}`)}
	entry := models.Entry{Key: "button.save", Description: "Saves the form"}

	create(t, db, &user)
	create(t, db, &locale)
	create(t, db, &entry)

	localization := models.Localization{EntryID: entry.ID, LocaleID: locale.ID, Translation: "Speichern"}
	create(t, db, &localization)

	translations, err := Translations(db, []string{localization.ID}, &user.ID, "")
	if err != nil {
		t.Fatalf("Translations() error = %v", err)
	}

	want := Translation{ID: localization.ID, EntryID: entry.ID, Key: "button.save", Locale: "de", Translation: "Speichern"}
	if translations.Count != 1 || len(translations.Translations) != 1 || translations.Translations[0] != want ||
		translations.User == nil || translations.User.Name != "Rita Reviewer" {
		t.Errorf("translations = %+v", translations)
	}

	entries, err := Entries(db, []string{entry.ID}, nil)
	if err != nil {
		t.Fatalf("Entries() error = %v", err)
	}

	if entries.Count != 1 || entries.Entries[0] != (Entry{ID: entry.ID, Key: "button.save", Description: "Saves the form"}) || entries.User != nil {
		t.Errorf("entries = %+v", entries)
	}

	// Long lists are cut, but still counted.
	ids := make([]string, maxItems+5)
	for i := range ids {
		ids[i] = entry.ID
	}

	entries, err = Entries(db, ids, nil)
	if err != nil || entries.Count != maxItems+5 || !entries.Truncated {
		t.Errorf("entries of %d ids = %+v, %v", len(ids), entries, err)
	}
}

func TestSendSignsTheBody(t *testing.T) {
	server, requests := receiver(t, http.StatusOK)
	box := testBox(t)
	sender := NewSender(box)

	encrypted, err := box.Encrypt("s3cret")
	if err != nil {
		t.Fatal(err)
	}

	hook := &models.Webhook{URL: server.URL, Secret: encrypted}
	delivery := &models.WebhookDelivery{ID: "delivery-1", Event: models.WebhookEventPing, Payload: `{"event":"ping"}`}

	result := sender.Send(context.Background(), hook, delivery)
	if result.Err != nil || result.Status != http.StatusOK || result.Body != "thanks" {
		t.Fatalf("result = %+v", result)
	}

	got := requests()[0]
	if string(got.body) != delivery.Payload ||
		got.header.Get("Content-Type") != "application/json" ||
		got.header.Get(HeaderEvent) != "ping" ||
		got.header.Get(HeaderDelivery) != "delivery-1" ||
		got.header.Get(HeaderSignature) != Sign("s3cret", got.body) {
		t.Errorf("request = %v %s", got.header, got.body)
	}

	if !strings.HasPrefix(got.header.Get(HeaderSignature), "sha256=") {
		t.Errorf("signature = %q", got.header.Get(HeaderSignature))
	}

	hook.Secret = ""
	sender.Send(context.Background(), hook, delivery)

	if signature := requests()[1].header.Get(HeaderSignature); signature != "" {
		t.Errorf("unsigned delivery has signature %q", signature)
	}
}

func TestSendFailures(t *testing.T) {
	sender := NewSender(testBox(t))
	delivery := &models.WebhookDelivery{Event: models.WebhookEventPing, Payload: `{}`}

	failing, _ := receiver(t, http.StatusInternalServerError)
	if result := sender.Send(context.Background(), &models.Webhook{URL: failing.URL}, delivery); result.Status != 500 || result.Err == nil ||
		!strings.Contains(result.Err.Error(), "500") || result.Body != "thanks" {
		t.Errorf("500: result = %+v", result)
	}

	redirecting := httptest.NewServer(http.RedirectHandler("http://example.test/elsewhere", http.StatusFound))
	defer redirecting.Close()

	if result := sender.Send(context.Background(), &models.Webhook{URL: redirecting.URL}, delivery); result.Status != http.StatusFound || result.Err == nil {
		t.Errorf("redirect: result = %+v, want it not followed", result)
	}

	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	address := closed.URL + "/hook?token=private"
	if result := sender.Send(context.Background(), &models.Webhook{URL: address}, delivery); result.Err == nil || strings.Contains(result.Err.Error(), "private") {
		t.Errorf("unreachable: result = %+v, want an error without the address", result)
	}

	if result := sender.Send(context.Background(), &models.Webhook{URL: failing.URL, Secret: "v1:garbage"}, delivery); result.Err != errSecretUnreadable {
		t.Errorf("unreadable secret: result = %+v", result)
	}
}

func TestDispatcherRetriesThenGivesUp(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *gorm.DB) {
		server, requests := receiver(t, http.StatusServiceUnavailable)
		hook := newHook(t, db, server.URL, models.WebhookEventEntriesCreated)
		dispatcher := NewDispatcher(db, NewSender(testBox(t)))

		emit(t, db, models.WebhookEventEntriesCreated, nil)
		dispatcher.Tick(context.Background())

		delivery := deliveriesOf(t, db, hook.ID)[0]
		if delivery.Status != models.DeliveryPending || delivery.Attempts != 1 || delivery.ResponseStatus != 503 ||
			!strings.Contains(delivery.Error, "503") || delivery.LastAttemptAt == nil {
			t.Fatalf("after the first attempt: %+v", delivery)
		}

		if wait := time.Until(*delivery.NextAttemptAt); wait < 50*time.Second || wait > time.Minute {
			t.Errorf("next attempt in %v, want a minute", wait)
		}

		// Not due yet, so a tick leaves it alone.
		dispatcher.Tick(context.Background())

		if len(requests()) != 1 {
			t.Fatalf("sent %d times before the retry was due", len(requests()))
		}

		// The last attempt fails it for good.
		db.Model(&delivery).Updates(map[string]any{"attempts": maxAttempts - 1, "next_attempt_at": time.Now().Add(-time.Second)})
		dispatcher.Tick(context.Background())

		delivery = deliveriesOf(t, db, hook.ID)[0]
		if delivery.Status != models.DeliveryFailed || delivery.Attempts != maxAttempts || delivery.NextAttemptAt != nil {
			t.Errorf("after the last attempt: %+v", delivery)
		}

		if got := requests(); len(got) != 2 || got[0].header.Get(HeaderDelivery) != delivery.ID || got[1].header.Get(HeaderDelivery) != delivery.ID {
			t.Errorf("requests = %d, want two of the same delivery", len(got))
		}
	})
}

func TestDispatcherDelivers(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *gorm.DB) {
		server, requests := receiver(t, http.StatusNoContent)
		hook := newHook(t, db, server.URL, models.WebhookEventEntriesCreated)

		emit(t, db, models.WebhookEventEntriesCreated, nil)
		NewDispatcher(db, NewSender(testBox(t))).Tick(context.Background())

		delivery := deliveriesOf(t, db, hook.ID)[0]
		if delivery.Status != models.DeliverySucceeded || delivery.Attempts != 1 || delivery.ResponseStatus != 204 ||
			delivery.Error != "" || delivery.NextAttemptAt != nil || len(requests()) != 1 {
			t.Errorf("delivery = %+v", delivery)
		}
	})
}

func TestDispatcherDropsDeliveriesOfGoneWebhooks(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *gorm.DB) {
		server, requests := receiver(t, http.StatusOK)
		off := newHook(t, db, server.URL, models.WebhookEventEntriesCreated)
		gone := newHook(t, db, server.URL, models.WebhookEventEntriesCreated)

		emit(t, db, models.WebhookEventEntriesCreated, nil)

		// As if they changed after the event: deleting usually takes the deliveries along.
		db.Model(&off).Update("enabled", false)
		db.Exec("DELETE FROM webhooks WHERE id = ?", gone.ID)

		NewDispatcher(db, NewSender(testBox(t))).Tick(context.Background())

		if len(requests()) != 0 {
			t.Errorf("sent %d deliveries", len(requests()))
		}

		if deliveries := deliveriesOf(t, db, off.ID); len(deliveries) != 1 || deliveries[0].Status != models.DeliveryFailed ||
			!strings.Contains(deliveries[0].Error, "turned off") {
			t.Errorf("deliveries of the webhook turned off = %+v", deliveries)
		}

		if deliveries := deliveriesOf(t, db, gone.ID); len(deliveries) != 0 {
			t.Errorf("deliveries of the deleted webhook = %+v", deliveries)
		}
	})
}

func TestClaimedDeliveriesAreSentOnce(t *testing.T) {
	db := testutil.SetupPostgresDB(t).DB

	var (
		mu    sync.Mutex
		times = map[string]int{}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		times[r.Header.Get(HeaderDelivery)]++
		mu.Unlock()
	}))
	defer server.Close()

	hook := newHook(t, db, server.URL, models.WebhookEventEntriesCreated)

	for range 25 {
		emit(t, db, models.WebhookEventEntriesCreated, nil)
	}

	var wg sync.WaitGroup

	for range 3 {
		dispatcher := NewDispatcher(db, NewSender(testBox(t)))

		wg.Go(func() { dispatcher.Tick(context.Background()) })
	}

	wg.Wait()

	for _, delivery := range deliveriesOf(t, db, hook.ID) {
		if delivery.Status != models.DeliverySucceeded || times[delivery.ID] != 1 {
			t.Errorf("delivery %s: %s, sent %d times", delivery.ID, delivery.Status, times[delivery.ID])
		}
	}

	if len(times) != 25 {
		t.Errorf("sent %d deliveries, want 25", len(times))
	}
}

func TestSendNowTriesOnceAndKeepsTheNewest(t *testing.T) {
	db := sqliteDB(t)
	server, _ := receiver(t, http.StatusBadGateway)
	hook := newHook(t, db, server.URL, models.WebhookEventEntriesCreated)

	// Ended deliveries beyond the newest keepDeliveries are dropped; pending ones stay.
	start := time.Now().Add(-time.Hour)
	for i := range keepDeliveries + 5 {
		create(t, db, &models.WebhookDelivery{
			WebhookID: hook.ID, EventID: "event", Event: models.WebhookEventEntriesCreated, Payload: "{}",
			Status: models.DeliverySucceeded, CreatedAt: start.Add(time.Duration(i) * time.Second),
		})
	}

	pending := models.WebhookDelivery{
		WebhookID: hook.ID, EventID: "event", Event: models.WebhookEventEntriesCreated, Payload: "{}",
		Status: models.DeliveryPending, CreatedAt: start.Add(-time.Hour),
	}
	create(t, db, &pending)

	delivery, err := Ping(&hook)
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	if err := NewSender(testBox(t)).SendNow(context.Background(), db, &hook, &delivery); err != nil {
		t.Fatalf("SendNow() error = %v", err)
	}

	if delivery.Status != models.DeliveryFailed || delivery.Attempts != 1 || delivery.ResponseStatus != 502 || delivery.NextAttemptAt != nil {
		t.Errorf("ping = %+v, want one failed attempt", delivery)
	}

	deliveries := deliveriesOf(t, db, hook.ID)
	if len(deliveries) != keepDeliveries+1 {
		t.Errorf("kept %d deliveries, want %d and the pending one", len(deliveries), keepDeliveries)
	}

	if deliveries[0].ID != pending.ID || deliveries[len(deliveries)-1].ID != delivery.ID {
		t.Error("dropped the pending delivery or the ping")
	}
}

func TestSendPostsInTheWebhooksFormat(t *testing.T) {
	server, requests := receiver(t, http.StatusNoContent)
	box := testBox(t)

	encrypted, err := box.Encrypt("s3cret")
	if err != nil {
		t.Fatal(err)
	}

	hook := &models.Webhook{Name: "Chat", URL: server.URL, Secret: encrypted, Format: models.WebhookFormatDiscord}

	delivery, err := Ping(hook)
	if err != nil {
		t.Fatal(err)
	}

	if result := NewSender(box).Send(context.Background(), hook, &delivery); result.Err != nil {
		t.Fatalf("result = %+v", result)
	}

	got := requests()[0]

	var body struct {
		Content string `json:"content"`
	}

	if err := json.Unmarshal(got.body, &body); err != nil || !strings.Contains(body.Content, "“Chat”") {
		t.Errorf("body = %s (%v), want a Discord message", got.body, err)
	}

	// The signature covers what was sent, not the stored event.
	if got.header.Get(HeaderSignature) != Sign("s3cret", got.body) || got.header.Get(HeaderEvent) != models.WebhookEventPing {
		t.Errorf("headers = %v", got.header)
	}
}

func TestScheduledWebhooksSendDigests(t *testing.T) {
	forEachDB(t, func(t *testing.T, db *gorm.DB) {
		server, requests := receiver(t, http.StatusOK)
		hook := models.Webhook{
			Name: "Daily", URL: server.URL, Enabled: true, Schedule: "0 9 * * *",
			Events: []string{models.WebhookEventEntriesCreated},
		}
		create(t, db, &hook)

		dispatcher := NewDispatcher(db, NewSender(testBox(t)))
		past := time.Now().Add(-time.Minute)
		due := func() {
			t.Helper()

			if err := db.Model(&hook).UpdateColumn("next_digest_at", past).Error; err != nil {
				t.Fatal(err)
			}
		}

		emit(t, db, models.WebhookEventEntriesCreated, EntriesData{Count: 1, Entries: []Entry{{Key: "one"}}})
		emit(t, db, models.WebhookEventEntriesCreated, EntriesData{Count: 2, Entries: []Entry{{Key: "two"}, {Key: "three"}}})
		dispatcher.Tick(context.Background())

		if deliveries := deliveriesOf(t, db, hook.ID); len(requests()) != 0 || len(deliveries) != 2 ||
			deliveries[0].Status != models.DeliveryCollected || deliveries[0].NextAttemptAt != nil {
			t.Fatalf("sent %d, deliveries = %+v; want both events collected", len(requests()), deliveries)
		}

		due()
		dispatcher.Tick(context.Background())

		if len(requests()) != 1 {
			t.Fatalf("sent %d requests, want one digest", len(requests()))
		}

		var body struct {
			Event string     `json:"event"`
			Data  DigestData `json:"data"`
		}

		if err := json.Unmarshal(requests()[0].body, &body); err != nil || body.Event != models.WebhookEventDigest ||
			body.Data.Count != 2 || len(body.Data.Events) != 2 || body.Data.Truncated || body.Data.Since.IsZero() {
			t.Errorf("digest = %s (%v)", requests()[0].body, err)
		}

		if deliveries := deliveriesOf(t, db, hook.ID); len(deliveries) != 1 || deliveries[0].Event != models.WebhookEventDigest ||
			deliveries[0].Status != models.DeliverySucceeded {
			t.Errorf("deliveries = %+v, want only the digest", deliveries)
		}

		var stored models.Webhook

		db.First(&stored, "id = ?", hook.ID)

		if stored.NextDigestAt == nil || !stored.NextDigestAt.After(time.Now()) || stored.NextDigestAt.UTC().Hour() != 9 {
			t.Errorf("next digest = %v, want the next 09:00 UTC", stored.NextDigestAt)
		}

		// Nothing collected, so nothing to send.
		due()
		dispatcher.Tick(context.Background())

		if len(requests()) != 1 {
			t.Errorf("sent %d requests, want no empty digest", len(requests()))
		}

		// Turning it off drops what it collected.
		emit(t, db, models.WebhookEventEntriesCreated, EntriesData{Count: 1})

		if err := FailPending(db, hook.ID); err != nil {
			t.Fatal(err)
		}

		if deliveries := deliveriesOf(t, db, hook.ID); deliveries[len(deliveries)-1].Status != models.DeliveryFailed {
			t.Errorf("collected event after turning off = %+v, want failed", deliveries[len(deliveries)-1])
		}
	})
}
