// Package webhookservice serves the WebhookService. Package webhook queues and sends
// the deliveries.
package webhookservice

import (
	"context"
	"net/url"
	"slices"
	"strings"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/taskqueue"
	"github.com/kazuto/parlance-server/internal/webhook"
)

// Limits of a webhook's settings
const (
	maxNameLength = 100
	maxURLLength  = 2000
)

// Server implements the WebhookService
type Server struct {
	db     *database.DB
	box    *secret.Box
	sender *webhook.Sender
}

// NewServer creates a new WebhookService server. Tests and redeliveries are sent
// with the sender.
func NewServer(db *database.DB, box *secret.Box, sender *webhook.Sender) parlancev1connect.WebhookServiceHandler {
	return &Server{db: db, box: box, sender: sender}
}

func (s *Server) findWebhook(ctx context.Context, id string) (models.Webhook, error) {
	if id == "" {
		return models.Webhook{}, errors.InvalidInput(errors.ErrInvalidInput, "webhook id is required")
	}

	return common.FindByID[models.Webhook](s.db.WithContext(ctx), id, errors.ErrWebhookNotFound, "webhook not found")
}

// webhooksToProto converts webhooks, each with its newest delivery and how many
// events it collected for its next digest. Collected events don't count as
// deliveries until their digest goes out.
func (s *Server) webhooksToProto(ctx context.Context, hooks []models.Webhook) ([]*pb.Webhook, error) {
	result := make([]*pb.Webhook, len(hooks))
	if len(hooks) == 0 {
		return result, nil
	}

	ids := make([]string, len(hooks))
	for i, hook := range hooks {
		ids[i] = hook.ID
	}

	var newest []models.WebhookDelivery
	if err := s.db.WithContext(ctx).Omit("payload", "response_body").
		Where("webhook_id IN ? AND status <> ?", ids, models.DeliveryCollected).
		Where("created_at = (SELECT MAX(newest.created_at) FROM webhook_deliveries newest WHERE newest.webhook_id = webhook_deliveries.webhook_id AND newest.status <> ?)", models.DeliveryCollected).
		Find(&newest).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load webhook deliveries")
	}

	var counts []struct {
		WebhookID string
		Count     int
	}

	if err := s.db.WithContext(ctx).Model(&models.WebhookDelivery{}).
		Select("webhook_id, COUNT(*) AS count").
		Where("webhook_id IN ? AND status = ?", ids, models.DeliveryCollected).
		Group("webhook_id").
		Scan(&counts).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to count collected events")
	}

	last := map[string]*models.WebhookDelivery{}
	for i := range newest {
		last[newest[i].WebhookID] = &newest[i]
	}

	collected := map[string]int{}
	for _, count := range counts {
		collected[count.WebhookID] = count.Count
	}

	for i := range hooks {
		result[i] = converter.WebhookToProto(&hooks[i], last[hooks[i].ID])
		result[i].CollectedCount = int32(collected[hooks[i].ID])
	}

	return result, nil
}

// webhookToProto converts one webhook as webhooksToProto does, leaving its
// deliveries out if they can't be loaded.
func (s *Server) webhookToProto(ctx context.Context, hook *models.Webhook) *pb.Webhook {
	result, err := s.webhooksToProto(ctx, []models.Webhook{*hook})
	if err != nil {
		return converter.WebhookToProto(hook, nil)
	}

	return result[0]
}

// encryptSecret returns the secret to store: encrypted, or empty for none.
func (s *Server) encryptSecret(plain string) (string, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return "", nil
	}

	encrypted, err := s.box.Encrypt(plain)
	if err != nil {
		return "", errors.FailedPrecondition(errors.ErrEncryptionKeyMissing, "set ENCRYPTION_KEY on the server to store signing secrets")
	}

	return encrypted, nil
}

func validateName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "name is required")
	}

	if len([]rune(name)) > maxNameLength {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "name is too long")
	}

	return name, nil
}

// validateURL accepts an absolute http or https URL. Addresses in private networks
// are fine, as receivers often run next to Parlance.
func validateURL(value string) (string, error) {
	address := strings.TrimSpace(value)

	parsed, err := url.Parse(address)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || len(address) > maxURLLength {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "url must be an http or https URL")
	}

	return address, nil
}

// validateSchedule accepts a standard 5-field cron expression, or none.
func validateSchedule(value string) (string, error) {
	schedule := strings.TrimSpace(value)
	if schedule == "" {
		return "", nil
	}

	if _, err := taskqueue.ParseSchedule(schedule); err != nil {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "schedule is not a valid cron expression")
	}

	return schedule, nil
}

// formatFromProto returns the format to post in; unspecified is JSON.
func formatFromProto(value pb.WebhookFormat) (string, error) {
	if value == pb.WebhookFormat_WEBHOOK_FORMAT_UNSPECIFIED {
		return models.WebhookFormatJSON, nil
	}

	format := converter.WebhookFormatFromProto(value)
	if format == "" {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "unknown format")
	}

	return format, nil
}

// deliveryToProto converts a delivery with the request body the webhook's format
// renders, and the answer's body.
func deliveryToProto(hook *models.Webhook, delivery *models.WebhookDelivery) *pb.WebhookDelivery {
	result := converter.WebhookDeliveryToProto(delivery, true)

	if body, err := webhook.Body(hook.Format, delivery.Payload); err == nil {
		result.Payload = string(body)
	}

	return result
}

// eventsFromProto checks the events a webhook subscribes to: at least one, and
// none it can't subscribe to.
func eventsFromProto(values []pb.WebhookEvent) ([]string, error) {
	events := []string{}

	for _, value := range values {
		event := converter.WebhookEventFromProto(value)
		if event == "" || event == models.WebhookEventPing || event == models.WebhookEventDigest {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "unknown event")
		}

		if !slices.Contains(events, event) {
			events = append(events, event)
		}
	}

	if len(events) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "choose at least one event")
	}

	return events, nil
}
