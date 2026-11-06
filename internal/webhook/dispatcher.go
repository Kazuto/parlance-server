package webhook

import (
	"context"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const (
	// defaultPollInterval is how often the dispatcher looks for due deliveries.
	defaultPollInterval = 5 * time.Second
	// batchSize is how many deliveries are sent at once.
	batchSize = 10
	// lease is how long a claimed delivery stays with the server sending it. Sending
	// takes at most the timeout, so a delivery still pending after the lease was
	// left by a server that stopped, and is sent again.
	lease = time.Minute
	// keepDeliveries is how many ended deliveries of each webhook are kept.
	keepDeliveries = 100
)

// Dispatcher sends the digests and deliveries that are due, deliveries in batches.
type Dispatcher struct {
	db     *gorm.DB
	sender *Sender

	pollInterval time.Duration
	wake         chan struct{}
}

// NewDispatcher creates a dispatcher; Start runs it.
func NewDispatcher(db *gorm.DB, sender *Sender) *Dispatcher {
	return &Dispatcher{
		// Polling would fill the log with its queries; slow ones and errors still show.
		db:           db.Session(&gorm.Session{Logger: db.Logger.LogMode(logger.Warn)}),
		sender:       sender,
		pollInterval: defaultPollInterval,
		wake:         make(chan struct{}, 1),
	}
}

// Wake makes the dispatcher look for deliveries now instead of at its next poll.
func (d *Dispatcher) Wake() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// Start runs the dispatcher until ctx ends. The returned channel closes once the
// deliveries in progress have ended; those cut short are sent again after their lease.
func (d *Dispatcher) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		ticker := time.NewTicker(d.pollInterval)
		defer ticker.Stop()

		for {
			d.Tick(ctx)

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-d.wake:
			}
		}
	}()

	return done
}

// Tick collects the digests that are due and sends every delivery that is due.
func (d *Dispatcher) Tick(ctx context.Context) {
	if err := d.collectDigests(ctx); err != nil {
		log.Printf("webhooks: collect digests: %v", err)
	}

	for ctx.Err() == nil {
		batch, err := d.claim(ctx)
		if err != nil {
			log.Printf("webhooks: claim deliveries: %v", err)

			return
		}

		if len(batch) == 0 {
			return
		}

		var wg sync.WaitGroup

		for i := range batch {
			wg.Go(func() { d.deliver(ctx, &batch[i]) })
		}

		wg.Wait()

		var webhookIDs []string

		for _, delivery := range batch {
			if !slices.Contains(webhookIDs, delivery.WebhookID) {
				webhookIDs = append(webhookIDs, delivery.WebhookID)
			}
		}

		for _, id := range webhookIDs {
			if err := prune(d.db, id); err != nil {
				log.Printf("webhooks: drop old deliveries of webhook %s: %v", id, err)
			}
		}
	}
}

// collectDigests turns what each webhook with a due digest collected into one
// delivery, and moves its next digest on. Webhooks another server is handling are
// skipped.
func (d *Dispatcher) collectDigests(ctx context.Context) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		var hooks []models.Webhook
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("enabled = ? AND next_digest_at <= ?", true, now).
			Find(&hooks).Error; err != nil {
			return err
		}

		for i := range hooks {
			hook := &hooks[i]

			if err := collectDigest(tx, hook); err != nil {
				return err
			}

			next, err := NextDigest(hook, now)
			if err != nil {
				// Saved schedules were checked, so this means a bad one got in;
				// stop collecting for it rather than every poll.
				log.Printf("webhooks: webhook %s has an invalid schedule: %v", hook.ID, err)
			}

			// Not an edit, so updated_at stays.
			if err := tx.Model(hook).UpdateColumn("next_digest_at", next).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// claim takes the next due deliveries, moving their next attempt past the lease so
// no other server sends them meanwhile.
func (d *Dispatcher) claim(ctx context.Context) ([]models.WebhookDelivery, error) {
	var batch []models.WebhookDelivery

	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_attempt_at <= ?", models.DeliveryPending, now).
			Order("next_attempt_at").
			Limit(batchSize).
			Find(&batch).Error; err != nil {
			return err
		}

		if len(batch) == 0 {
			return nil
		}

		ids := make([]string, len(batch))
		next := now.Add(lease)

		for i := range batch {
			ids[i] = batch[i].ID
			batch[i].NextAttemptAt = &next
		}

		return tx.Model(&models.WebhookDelivery{}).Where("id IN ?", ids).Update("next_attempt_at", next).Error
	})

	return batch, err
}

// deliver sends a claimed delivery and records how it went.
func (d *Dispatcher) deliver(ctx context.Context, delivery *models.WebhookDelivery) {
	var hooks []models.Webhook
	if err := d.db.WithContext(ctx).Where("id = ?", delivery.WebhookID).Limit(1).Find(&hooks).Error; err != nil {
		log.Printf("webhooks: load webhook %s: %v", delivery.WebhookID, err)

		return
	}

	switch {
	case len(hooks) == 0:
		// The webhook was deleted, which usually takes its deliveries along.
		if err := d.db.Delete(delivery).Error; err != nil {
			log.Printf("webhooks: drop delivery %s: %v", delivery.ID, err)
		}

		return
	case !hooks[0].Enabled:
		if err := FailPending(d.db, delivery.WebhookID); err != nil {
			log.Printf("webhooks: fail deliveries of webhook %s: %v", delivery.WebhookID, err)
		}

		return
	}

	result := d.sender.Send(ctx, &hooks[0], delivery)

	// Cut short by a shutdown: the lease runs out and the delivery is sent again.
	if ctx.Err() != nil {
		return
	}

	if err := record(d.db, delivery, result, true); err != nil {
		log.Printf("webhooks: record delivery %s: %v", delivery.ID, err)
	}
}

// prune drops the webhook's oldest ended deliveries beyond keepDeliveries; pending
// and collected ones stay.
func prune(db *gorm.DB, webhookID string) error {
	kept := db.Model(&models.WebhookDelivery{}).Select("id").
		Where("webhook_id = ?", webhookID).
		Order("created_at DESC").
		Limit(keepDeliveries)

	return db.
		Where("webhook_id = ? AND status NOT IN ? AND id NOT IN (?)", webhookID, []string{models.DeliveryPending, models.DeliveryCollected}, kept).
		Delete(&models.WebhookDelivery{}).Error
}
