package auth

import (
	"context"
	"log"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/webhook"
)

// Limits of login.failed events, so guessing passwords can't flood the webhooks
const (
	// loginWindow is how long more failures with an email from an address are only
	// counted after an event about them, and how long maxLoginEvents holds.
	loginWindow = 15 * time.Minute
	// maxLoginEvents caps the events of a window across all emails and addresses.
	maxLoginEvents = 30
	// maxLoginKeys caps the emails and addresses remembered.
	maxLoginKeys = 10000
	// maxUserAgent caps the user agent sent along.
	maxUserAgent = 256
)

// loginReporter sends failed sign-ins to the webhooks subscribed to login.failed:
// the first failure with an email from an address right away, and more of them at
// most once per window, counting the attempts since the last event. Attempts at the
// end of a run of failures stay uncounted when no failure follows within a window.
type loginReporter struct {
	db      *database.DB
	trusted []netip.Prefix
	now     func() time.Time

	mu sync.Mutex
	// failures are by lower-case email and address.
	failures map[string]*loginFailures
	// sent holds when the window's events went out, oldest first.
	sent     []time.Time
	prunedAt time.Time
}

// loginFailures are the failed sign-ins with an email from an address.
type loginFailures struct {
	// reportedAt is when the last event about them went out.
	reportedAt time.Time
	lastAt     time.Time
	// unreported counts the failures since that event.
	unreported int
}

func newLoginReporter(db *database.DB, trusted []netip.Prefix) *loginReporter {
	return &loginReporter{db: db, trusted: trusted, now: time.Now, failures: map[string]*loginFailures{}}
}

// report notes a failed sign-in from the request with its peer and headers, and
// sends an event when one is due. It never fails the sign-in's response, and logs
// what goes wrong instead.
func (r *loginReporter) report(ctx context.Context, peer string, header http.Header, data webhook.LoginFailedData) {
	if r == nil {
		return
	}

	data.IP = middleware.ClientIP(peer, header.Values("X-Forwarded-For"), r.trusted)
	data.UserAgent = header.Get("User-Agent")

	if len(data.UserAgent) > maxUserAgent {
		data.UserAgent = strings.ToValidUTF8(data.UserAgent[:maxUserAgent], "")
	}

	attempts, due := r.count(strings.ToLower(data.Email) + " " + data.IP)
	if !due {
		return
	}

	data.Attempts = attempts
	db := r.db.WithContext(context.WithoutCancel(ctx))

	if err := webhook.Emit(db, models.WebhookEventLoginFailed, func() (any, error) {
		return data, nil
	}); err != nil {
		log.Printf("auth: report a failed sign-in: %v", err)
	}
}

// count notes a failure with the key and tells whether an event is due, and how
// many failures it covers.
func (r *loginReporter) count(key string) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	r.prune(now)

	failures := r.failures[key]
	if failures == nil {
		if len(r.failures) >= maxLoginKeys {
			return 0, false
		}

		failures = &loginFailures{}
		r.failures[key] = failures
	}

	failures.lastAt = now
	failures.unreported++

	if now.Sub(failures.reportedAt) < loginWindow || len(r.sent) >= maxLoginEvents {
		return 0, false
	}

	attempts := failures.unreported
	failures.reportedAt = now
	failures.unreported = 0

	r.sent = append(r.sent, now)

	return attempts, true
}

// prune forgets events older than a window, and, at most once a minute, failures
// with no other failure for a window.
func (r *loginReporter) prune(now time.Time) {
	for len(r.sent) > 0 && now.Sub(r.sent[0]) >= loginWindow {
		r.sent = r.sent[1:]
	}

	if now.Sub(r.prunedAt) < time.Minute {
		return
	}

	r.prunedAt = now

	for key, failures := range r.failures {
		if now.Sub(failures.lastAt) >= loginWindow {
			delete(r.failures, key)
		}
	}
}
