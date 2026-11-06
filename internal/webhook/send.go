package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"gorm.io/gorm"
)

// Headers of every delivery; the signature is only sent with a secret.
const (
	HeaderEvent     = "X-Parlance-Event"
	HeaderDelivery  = "X-Parlance-Delivery"
	HeaderSignature = "X-Parlance-Signature"
	userAgent       = "Parlance-Webhook"
)

const (
	// timeout is how long a receiver may take to answer.
	timeout = 10 * time.Second
	// maxResponseBody is how much of an answer is kept, to show what went wrong.
	maxResponseBody = 4 << 10
)

// retryDelays are the waits before the attempts after the first, so a delivery is
// tried for about seven hours before it fails.
var retryDelays = []time.Duration{time.Minute, 10 * time.Minute, time.Hour, 6 * time.Hour}

// maxAttempts is how often a delivery is tried.
var maxAttempts = len(retryDelays) + 1

var errSecretUnreadable = stderrors.New("the signing secret can't be read, e.g. because ENCRYPTION_KEY changed; enter it again")

// Sign returns the signature of a body as the X-Parlance-Signature header carries
// it: "sha256=" and the hex HMAC-SHA256 of the body, keyed with the secret.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)

	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Result is how an attempt went: without Err, the receiver took the delivery.
type Result struct {
	// Status is the HTTP status of the answer, 0 without one.
	Status   int
	Body     string
	Duration time.Duration
	Err      error
}

// Sender posts deliveries to their webhooks.
type Sender struct {
	client *http.Client
	box    *secret.Box
}

// NewSender creates a sender that decrypts the webhooks' secrets with the box.
func NewSender(box *secret.Box) *Sender {
	return &Sender{
		box: box,
		client: &http.Client{
			Timeout: timeout,
			// The answer to the first request counts; receivers are set up with their
			// final address.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Send posts the delivery to the webhook once. Answers outside 2xx count as failed.
func (s *Sender) Send(ctx context.Context, hook *models.Webhook, delivery *models.WebhookDelivery) Result {
	started := time.Now()
	result := s.send(ctx, hook, delivery)
	result.Duration = time.Since(started)

	return result
}

func (s *Sender) send(ctx context.Context, hook *models.Webhook, delivery *models.WebhookDelivery) Result {
	body, err := Body(hook.Format, delivery.Payload)
	if err != nil {
		return Result{Err: err}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		return Result{Err: err}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set(HeaderEvent, delivery.Event)
	req.Header.Set(HeaderDelivery, delivery.ID)

	if hook.Secret != "" {
		plain, err := s.box.Decrypt(hook.Secret)
		if err != nil {
			return Result{Err: errSecretUnreadable}
		}

		req.Header.Set(HeaderSignature, Sign(plain, body))
	}

	res, err := s.client.Do(req)
	if err != nil {
		// The address is shown next to the delivery already, and may hold a token.
		var urlErr *url.Error
		if stderrors.As(err, &urlErr) {
			err = urlErr.Err
		}

		return Result{Err: err}
	}

	defer func() { _ = res.Body.Close() }()

	// The start of the answer shows what went wrong; one that can't be read is left out.
	answer, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBody))
	if err != nil {
		answer = nil
	}

	result := Result{Status: res.StatusCode, Body: cleanText(answer)}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		result.Err = fmt.Errorf("the receiver answered %s", res.Status)
	}

	return result
}

// SendNow saves the delivery and sends it right away, once: a failure is not tried
// again. Should the server stop before the answer, the dispatcher sends it later.
func (s *Sender) SendNow(ctx context.Context, db *gorm.DB, hook *models.Webhook, delivery *models.WebhookDelivery) error {
	next := time.Now().Add(lease)
	delivery.Status = models.DeliveryPending
	delivery.NextAttemptAt = &next

	if err := db.WithContext(ctx).Create(delivery).Error; err != nil {
		return err
	}

	if err := record(db.WithContext(ctx), delivery, s.Send(ctx, hook, delivery), false); err != nil {
		return err
	}

	return prune(db.WithContext(ctx), hook.ID)
}

// record saves how an attempt went. A failed attempt is tried again later while
// retry is set and attempts remain; otherwise the delivery fails.
func record(db *gorm.DB, delivery *models.WebhookDelivery, result Result, retry bool) error {
	now := time.Now()

	delivery.Attempts++
	delivery.LastAttemptAt = &now
	delivery.ResponseStatus = result.Status
	delivery.ResponseBody = result.Body
	delivery.DurationMs = int(result.Duration.Milliseconds())
	delivery.NextAttemptAt = nil
	delivery.Error = ""

	switch {
	case result.Err == nil:
		delivery.Status = models.DeliverySucceeded
	case retry && delivery.Attempts < maxAttempts:
		next := now.Add(retryDelays[delivery.Attempts-1])
		delivery.Status = models.DeliveryPending
		delivery.NextAttemptAt = &next
		delivery.Error = result.Err.Error()
	default:
		delivery.Status = models.DeliveryFailed
		delivery.Error = result.Err.Error()
	}

	// A delivery failed meanwhile, e.g. as its webhook was turned off, stays failed.
	return db.Model(delivery).
		Where("status = ?", models.DeliveryPending).
		Select("status", "attempts", "next_attempt_at", "last_attempt_at", "response_status", "response_body", "error", "duration_ms").
		Updates(delivery).Error
}

// FailPending ends the pending and collected deliveries of a webhook that was turned
// off as failed.
func FailPending(tx *gorm.DB, webhookID string) error {
	return tx.Model(&models.WebhookDelivery{}).
		Where("webhook_id = ? AND status IN ?", webhookID, []string{models.DeliveryPending, models.DeliveryCollected}).
		Updates(map[string]any{
			"status":          models.DeliveryFailed,
			"next_attempt_at": nil,
			"error":           "the webhook was turned off before the delivery was sent",
		}).Error
}

// cleanText makes an answer storable as text: valid UTF-8, without NUL bytes.
func cleanText(raw []byte) string {
	return strings.ToValidUTF8(strings.ReplaceAll(string(raw), "\x00", ""), "�")
}
