package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	authpkg "github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/webhook"
)

// loginHook subscribes a webhook to login.failed and returns what it was sent, and
// the payloads as they are stored.
func loginHook(t *testing.T, db *database.DB) func() ([]webhook.LoginFailedData, string) {
	t.Helper()

	hook := models.Webhook{
		Name: "Security", URL: "https://chat.example/hook", Enabled: true,
		Events: []string{models.WebhookEventLoginFailed},
	}
	if err := db.Create(&hook).Error; err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		db.Exec(`DELETE FROM webhook_deliveries WHERE webhook_id = ?`, hook.ID)
		db.Exec(`DELETE FROM webhooks WHERE id = ?`, hook.ID)
	})

	return func() ([]webhook.LoginFailedData, string) {
		t.Helper()

		var deliveries []models.WebhookDelivery

		db.Where("webhook_id = ?", hook.ID).Order("created_at").Find(&deliveries)

		sent := make([]webhook.LoginFailedData, len(deliveries))
		payloads := ""

		for i, delivery := range deliveries {
			var body struct {
				Data webhook.LoginFailedData `json:"data"`
			}

			if err := json.Unmarshal([]byte(delivery.Payload), &body); err != nil {
				t.Fatalf("payload: %v", err)
			}

			sent[i] = body.Data
			payloads += delivery.Payload
		}

		return sent, payloads
	}
}

func TestFailedPasswordSignInsAreReported(t *testing.T) {
	_, db := testutil.Setup(t)
	sent := loginHook(t, db)

	hash, err := authpkg.HashPassword("pw-right")
	if err != nil {
		t.Fatal(err)
	}

	user := models.User{Email: "rita@example.com", Name: "Rita", PasswordHash: hash}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { db.Exec(`DELETE FROM users WHERE id = ?`, user.ID) })

	server := NewServer(db, &config.Config{JWT: config.JWTConfig{Secret: "jwt-secret", Expiry: "1h"}}, nil).(*Server)
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	server.failures.now = func() time.Time { return now }

	login := func(email, password string) error {
		req := connect.NewRequest(&pb.LoginRequest{Email: email, Password: password})
		req.Header().Set("User-Agent", "curl/8.0")
		_, err := server.Login(context.Background(), req)

		return err
	}

	fail := func(email, password string) {
		t.Helper()

		if err := login(email, password); connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Fatalf("Login(%s, %s) error = %v, want unauthenticated", email, password, err)
		}
	}

	fail("nobody@example.com", "pw-guess")
	fail("rita@example.com", "pw-one")
	fail("rita@example.com", "pw-two")
	fail("rita@example.com", "pw-three")

	if err := login("rita@example.com", "pw-right"); err != nil {
		t.Fatalf("Login() with the right password: %v", err)
	}

	// Only the first failure with each email is sent right away.
	got, _ := sent()
	want := []webhook.LoginFailedData{
		{Email: "nobody@example.com", Method: webhook.LoginMethodPassword, Reason: webhook.LoginReasonUnknownEmail, UserAgent: "curl/8.0", Attempts: 1},
		{Email: "rita@example.com", Method: webhook.LoginMethodPassword, Reason: webhook.LoginReasonWrongPassword, UserAgent: "curl/8.0", Attempts: 1},
	}

	if !slices.Equal(got, want) {
		t.Fatalf("sent %+v\nwant %+v", got, want)
	}

	// While the failures go on, the next event comes a window after the last one and
	// counts the failures since.
	now = now.Add(10 * time.Minute)

	fail("rita@example.com", "pw-four")

	now = now.Add(5 * time.Minute)

	fail("rita@example.com", "pw-five")

	got, payloads := sent()
	if len(got) != 3 || got[2].Attempts != 4 {
		t.Fatalf("sent %+v, want a third event counting 4 attempts", got)
	}

	if strings.Contains(payloads, "pw-") {
		t.Errorf("a payload holds a password: %s", payloads)
	}
}

func TestLoginReporterLimits(t *testing.T) {
	ctx, db := testutil.Setup(t)
	sent := loginHook(t, db)

	reporter := newLoginReporter(db, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	reporter.now = func() time.Time { return now }

	failed := func(email string) webhook.LoginFailedData {
		return webhook.LoginFailedData{Email: email, Method: webhook.LoginMethodPassword, Reason: webhook.LoginReasonWrongPassword}
	}

	// Behind a trusted proxy, the address is the client's.
	header := http.Header{"X-Forwarded-For": {"198.51.100.1"}, "User-Agent": {strings.Repeat("x", 300)}}
	reporter.report(ctx, "10.0.0.2:4000", header, failed("Rita@Example.com"))

	got, _ := sent()
	if len(got) != 1 || got[0].IP != "198.51.100.1" || len(got[0].UserAgent) != maxUserAgent {
		t.Fatalf("sent %+v, want the client's address and a shortened user agent", got)
	}

	// The email in other case from the same address is counted, not sent; from
	// another address, it's sent.
	reporter.report(ctx, "10.0.0.2:4000", header, failed("rita@example.com"))
	reporter.report(ctx, "203.0.113.9:1234", nil, failed("rita@example.com"))

	if got, _ = sent(); len(got) != 2 || got[1].IP != "203.0.113.9" {
		t.Fatalf("sent %+v, want a second event from the other address", got)
	}

	// All emails and addresses together send at most maxLoginEvents in a window.
	for i := range maxLoginEvents {
		reporter.report(ctx, "203.0.113.9:1234", nil, failed(fmt.Sprintf("user%d@example.com", i)))
	}

	if got, _ = sent(); len(got) != maxLoginEvents {
		t.Fatalf("sent %d events in a window, want %d", len(got), maxLoginEvents)
	}

	// A window later, events go out again. Failures with nothing after them for a
	// window are forgotten, so they start counting anew.
	now = now.Add(loginWindow)

	reporter.report(ctx, "10.0.0.2:4000", header, failed("rita@example.com"))

	if got, _ = sent(); len(got) != maxLoginEvents+1 || got[maxLoginEvents].Attempts != 1 {
		t.Fatalf("sent %+v after a window, want one more event counting 1 attempt", got[len(got)-1])
	}

	// Without a reporter, nothing is reported.
	var none *loginReporter

	none.report(ctx, "203.0.113.9:1234", nil, failed("rita@example.com"))
}
