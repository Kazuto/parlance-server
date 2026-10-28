package models

import (
	"time"

	"gorm.io/datatypes"
)

// Events a webhook can subscribe to, as receivers see them
const (
	WebhookEventTranslationsApproved = "translations.approved"
	WebhookEventTranslationsRejected = "translations.rejected"
	// WebhookEventTranslationsAwaitingApproval is sent when translations become drafts.
	WebhookEventTranslationsAwaitingApproval = "translations.awaiting_approval"
	WebhookEventEntriesCreated               = "entries.created"
	WebhookEventTaskRunSucceeded             = "task_run.succeeded"
	WebhookEventTaskRunFailed                = "task_run.failed"
	// WebhookEventPing is only sent by a test, so nothing subscribes to it.
	WebhookEventPing = "ping"
	// WebhookEventDigest carries what a webhook with a schedule collected.
	WebhookEventDigest = "digest"
)

// How a webhook posts events: the event as JSON, or a chat message describing it
const (
	WebhookFormatJSON    = "json"
	WebhookFormatSlack   = "slack"
	WebhookFormatDiscord = "discord"
)

// Webhook delivery states
const (
	DeliveryPending   = "pending"
	DeliverySucceeded = "succeeded"
	DeliveryFailed    = "failed"
	// DeliveryCollected waits for the webhook's next digest instead of being sent.
	DeliveryCollected = "collected"
)

// Webhook posts the events it subscribes to to a URL.
type Webhook struct {
	ID      string                      `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Name    string                      `gorm:"type:varchar(100);not null" json:"name"`
	URL     string                      `gorm:"type:text;not null" json:"url"`
	Events  datatypes.JSONSlice[string] `gorm:"type:jsonb" json:"events"`
	Enabled bool                        `gorm:"not null" json:"enabled"`
	Format  string                      `gorm:"type:varchar(20);not null;default:'json'" json:"format"`
	// Schedule is a standard 5-field cron expression in UTC on which the collected
	// events go out as one digest; empty to send each event right away.
	Schedule string `gorm:"type:varchar(100);not null;default:''" json:"schedule"`
	// NextDigestAt is when the next digest goes out, also right away once a webhook
	// that collected events loses its schedule.
	NextDigestAt *time.Time `gorm:"index" json:"next_digest_at,omitempty"`
	// Secret signs the deliveries; it is encrypted with the server's encryption key.
	Secret    string    `gorm:"type:text" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy *string   `gorm:"type:uuid" json:"created_by,omitempty"`
	UpdatedBy *string   `gorm:"type:uuid" json:"updated_by,omitempty"`
}

// WebhookDelivery is one event sent to one webhook, from being queued until it was
// delivered or given up on.
type WebhookDelivery struct {
	ID        string   `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	WebhookID string   `gorm:"type:uuid;not null;index" json:"webhook_id"`
	Webhook   *Webhook `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	// EventID is the same in every delivery of the event, redeliveries included.
	EventID string `gorm:"type:uuid;not null" json:"event_id"`
	Event   string `gorm:"type:varchar(50);not null" json:"event"`
	// Payload is the event as JSON; the webhook's format renders the request body
	// from it when sending, so a changed format applies to retries too.
	Payload  string `gorm:"type:text;not null" json:"payload"`
	Status   string `gorm:"type:varchar(20);not null;index:idx_webhook_deliveries_due,priority:1" json:"status"`
	Attempts int    `gorm:"not null;default:0" json:"attempts"`
	// NextAttemptAt is when a pending delivery is tried next. A server sending it
	// moves it on, so it is tried again should that server stop.
	NextAttemptAt  *time.Time `gorm:"index:idx_webhook_deliveries_due,priority:2" json:"next_attempt_at,omitempty"`
	LastAttemptAt  *time.Time `json:"last_attempt_at,omitempty"`
	ResponseStatus int        `gorm:"not null;default:0" json:"response_status"`
	ResponseBody   string     `gorm:"type:text;not null;default:''" json:"response_body"`
	Error          string     `gorm:"type:text;not null;default:''" json:"error"`
	DurationMs     int        `gorm:"not null;default:0" json:"duration_ms"`
	CreatedAt      time.Time  `json:"created_at"`
}
