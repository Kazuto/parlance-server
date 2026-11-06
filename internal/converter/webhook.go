package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// Model values and their proto enums
var (
	webhookEvents = map[string]pb.WebhookEvent{
		models.WebhookEventTranslationsApproved: pb.WebhookEvent_WEBHOOK_EVENT_TRANSLATIONS_APPROVED,
		models.WebhookEventTranslationsRejected: pb.WebhookEvent_WEBHOOK_EVENT_TRANSLATIONS_REJECTED,
		models.WebhookEventEntriesCreated:       pb.WebhookEvent_WEBHOOK_EVENT_ENTRIES_CREATED,
		models.WebhookEventTaskRunSucceeded:     pb.WebhookEvent_WEBHOOK_EVENT_TASK_RUN_SUCCEEDED,
		models.WebhookEventTaskRunFailed:        pb.WebhookEvent_WEBHOOK_EVENT_TASK_RUN_FAILED,
		models.WebhookEventPing:                 pb.WebhookEvent_WEBHOOK_EVENT_PING,
		models.WebhookEventDigest:               pb.WebhookEvent_WEBHOOK_EVENT_DIGEST,

		models.WebhookEventTranslationsAwaitingApproval: pb.WebhookEvent_WEBHOOK_EVENT_TRANSLATIONS_AWAITING_APPROVAL,
	}
	webhookFormats = map[string]pb.WebhookFormat{
		models.WebhookFormatJSON:    pb.WebhookFormat_WEBHOOK_FORMAT_JSON,
		models.WebhookFormatSlack:   pb.WebhookFormat_WEBHOOK_FORMAT_SLACK,
		models.WebhookFormatDiscord: pb.WebhookFormat_WEBHOOK_FORMAT_DISCORD,
	}
	deliveryStatuses = map[string]pb.WebhookDeliveryStatus{
		models.DeliveryPending:   pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_PENDING,
		models.DeliverySucceeded: pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_SUCCEEDED,
		models.DeliveryFailed:    pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_FAILED,
		models.DeliveryCollected: pb.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_COLLECTED,
	}
)

// WebhookEventFromProto returns the model's event name, or "" for none.
func WebhookEventFromProto(value pb.WebhookEvent) string { return lookup(webhookEvents, value) }

// WebhookFormatFromProto returns the model's format, or "" for none.
func WebhookFormatFromProto(value pb.WebhookFormat) string { return lookup(webhookFormats, value) }

// WebhookToProto converts a Webhook with its newest delivery, if any. The secret
// stays out: only whether one is set is sent. The caller adds the collected count.
func WebhookToProto(hook *models.Webhook, lastDelivery *models.WebhookDelivery) *pb.Webhook {
	result := &pb.Webhook{
		Id:           hook.ID,
		Name:         hook.Name,
		Url:          hook.URL,
		Enabled:      hook.Enabled,
		HasSecret:    hook.Secret != "",
		Format:       webhookFormats[hook.Format],
		Schedule:     hook.Schedule,
		NextDigestAt: optionalTime(hook.NextDigestAt),
		CreatedAt:    hook.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    hook.UpdatedAt.Format(time.RFC3339),
	}

	for _, event := range hook.Events {
		if value, ok := webhookEvents[event]; ok {
			result.Events = append(result.Events, value)
		}
	}

	if lastDelivery != nil {
		result.LastDelivery = WebhookDeliveryToProto(lastDelivery, false)
	}

	return result
}

// WebhookDeliveryToProto converts a delivery, with its payload and the answer's body
// only when asked, as they are long.
func WebhookDeliveryToProto(delivery *models.WebhookDelivery, withBodies bool) *pb.WebhookDelivery {
	result := &pb.WebhookDelivery{
		Id:             delivery.ID,
		WebhookId:      delivery.WebhookID,
		Event:          webhookEvents[delivery.Event],
		Status:         deliveryStatuses[delivery.Status],
		Attempts:       int32(delivery.Attempts),
		NextAttemptAt:  optionalTime(delivery.NextAttemptAt),
		LastAttemptAt:  optionalTime(delivery.LastAttemptAt),
		ResponseStatus: int32(delivery.ResponseStatus),
		Error:          delivery.Error,
		DurationMs:     int32(delivery.DurationMs),
		CreatedAt:      delivery.CreatedAt.Format(time.RFC3339),
	}

	if withBodies {
		result.Payload = delivery.Payload
		result.ResponseBody = delivery.ResponseBody
	}

	return result
}
