// Package webhook sends events to the webhooks subscribed to them. Changes queue
// deliveries in their own transaction (Emit), and the Dispatcher posts them, trying
// failed ones again for a few hours. Webhooks with a schedule collect their events
// instead, and the Dispatcher sends what they collected as one digest. Several
// servers may dispatch from one database: each delivery and digest is claimed with
// row locks, so it is sent once at a time.
package webhook

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/taskqueue"
	"gorm.io/gorm"
)

const (
	// maxItems caps the translations or entries an event lists; Count tells how
	// many it is about.
	maxItems = 1000
	// maxDigestBytes caps the events a digest carries, as bulk changes make long ones.
	maxDigestBytes = 1 << 20
)

// envelope is the body of every delivery.
type envelope struct {
	// ID is the event's, the same in every delivery of it, so receivers can skip
	// events they have seen.
	ID        string    `json:"id"`
	Event     string    `json:"event"`
	CreatedAt time.Time `json:"created_at"`
	Data      any       `json:"data"`
}

// User is who caused an event; it is null for changes nobody made by hand.
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Translation is a translation an event is about.
type Translation struct {
	ID          string `json:"id"`
	EntryID     string `json:"entry_id"`
	Key         string `json:"key"`
	Locale      string `json:"locale"`
	Translation string `json:"translation"`
}

// TranslationsData is the data of translations.approved and translations.rejected.
type TranslationsData struct {
	Count int `json:"count"`
	// Translations lists the first maxItems of them, by key and locale.
	Translations []Translation `json:"translations"`
	Truncated    bool          `json:"truncated"`
	// Comment is why the reviewer rejected them.
	Comment string `json:"comment,omitempty"`
	User    *User  `json:"user"`
	// Task drafted them by machine translation.
	Task *TaskInfo `json:"task,omitempty"`
}

// Entry is an entry an event is about.
type Entry struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Description string `json:"description"`
}

// EntriesData is the data of entries.created.
type EntriesData struct {
	Count int `json:"count"`
	// Entries lists the first maxItems of them, by key.
	Entries   []Entry `json:"entries"`
	Truncated bool    `json:"truncated"`
	User      *User   `json:"user"`
}

// TaskRunData is the data of task_run.succeeded and task_run.failed.
type TaskRunData struct {
	Task TaskInfo `json:"task"`
	Run  RunInfo  `json:"run"`
}

// TaskInfo is the task of a run.
type TaskInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// RunInfo is a run that ended.
type RunInfo struct {
	ID         string                `json:"id"`
	Trigger    string                `json:"trigger"`
	Status     string                `json:"status"`
	StartedAt  *time.Time            `json:"started_at"`
	FinishedAt *time.Time            `json:"finished_at"`
	Error      string                `json:"error"`
	Summary    models.TaskRunSummary `json:"summary"`
	// User started the run by hand.
	User *User `json:"user"`
}

// DigestData is the data of a digest: the events a webhook collected, oldest first.
type DigestData struct {
	// Since is when the first of them happened.
	Since time.Time `json:"since"`
	Count int       `json:"count"`
	// Events holds each event's body as a webhook without a schedule gets it, up to
	// a megabyte of them.
	Events    []json.RawMessage `json:"events"`
	Truncated bool              `json:"truncated"`
}

// PingData is the data of a test delivery.
type PingData struct {
	Webhook struct {
		ID     string   `json:"id"`
		Name   string   `json:"name"`
		Events []string `json:"events"`
	} `json:"webhook"`
}

// Emit queues a delivery of the event to each enabled webhook subscribed to it, or
// collects it for the webhook's next digest. It runs in the transaction of the
// change, so a change that is rolled back sends nothing. data is only called when
// a webhook wants the event.
func Emit(tx *gorm.DB, event string, data func() (any, error)) error {
	var hooks []models.Webhook
	if err := tx.Where("enabled = ?", true).Find(&hooks).Error; err != nil {
		return err
	}

	hooks = slices.DeleteFunc(hooks, func(hook models.Webhook) bool {
		return !slices.Contains(hook.Events, event)
	})
	if len(hooks) == 0 {
		return nil
	}

	value, err := data()
	if err != nil {
		return err
	}

	eventID := uuid.NewString()

	payload, err := render(eventID, event, value)
	if err != nil {
		return err
	}

	now := time.Now()
	deliveries := make([]models.WebhookDelivery, len(hooks))

	for i, hook := range hooks {
		deliveries[i] = models.WebhookDelivery{
			WebhookID:     hook.ID,
			EventID:       eventID,
			Event:         event,
			Payload:       payload,
			Status:        models.DeliveryPending,
			NextAttemptAt: &now,
		}

		if hook.Schedule != "" {
			deliveries[i].Status = models.DeliveryCollected
			deliveries[i].NextAttemptAt = nil
		}
	}

	return tx.Create(&deliveries).Error
}

// EmitTranslations sends an event about the translations with the ids, done by the
// user or the task, if any.
func EmitTranslations(tx *gorm.DB, event string, ids []string, userID *string, task *models.Task) error {
	if len(ids) == 0 {
		return nil
	}

	return Emit(tx, event, func() (any, error) {
		data, err := Translations(tx, ids, userID, "")
		if task != nil {
			data.Task = &TaskInfo{ID: task.ID, Name: task.Name, Type: task.Type}
		}

		return data, err
	})
}

// NextDigest returns when the webhook's schedule sends its next digest after the
// time, or nil while it is off or has no schedule.
func NextDigest(hook *models.Webhook, after time.Time) (*time.Time, error) {
	if !hook.Enabled || hook.Schedule == "" {
		return nil, nil
	}

	schedule, err := taskqueue.ParseSchedule(hook.Schedule)
	if err != nil {
		return nil, err
	}

	next := schedule.Next(after.UTC())

	return &next, nil
}

// collectDigest turns what the webhook collected into one digest delivery, sent
// right away. Without collected events, there is no digest.
func collectDigest(tx *gorm.DB, hook *models.Webhook) error {
	var collected []models.WebhookDelivery
	if err := tx.Where("webhook_id = ? AND status = ?", hook.ID, models.DeliveryCollected).
		Order("created_at").
		Find(&collected).Error; err != nil {
		return err
	}

	if len(collected) == 0 {
		return nil
	}

	data := DigestData{Since: collected[0].CreatedAt.UTC(), Count: len(collected), Events: []json.RawMessage{}}
	size := 0
	ids := make([]string, len(collected))

	for i, delivery := range collected {
		ids[i] = delivery.ID

		// The first event goes along, however long it is.
		if size += len(delivery.Payload); data.Truncated || (size > maxDigestBytes && i > 0) {
			data.Truncated = true

			continue
		}

		data.Events = append(data.Events, json.RawMessage(delivery.Payload))
	}

	eventID := uuid.NewString()

	payload, err := render(eventID, models.WebhookEventDigest, data)
	if err != nil {
		return err
	}

	now := time.Now()
	digest := models.WebhookDelivery{
		WebhookID:     hook.ID,
		EventID:       eventID,
		Event:         models.WebhookEventDigest,
		Payload:       payload,
		Status:        models.DeliveryPending,
		NextAttemptAt: &now,
	}

	if err := tx.Create(&digest).Error; err != nil {
		return err
	}

	return tx.Where("id IN ?", ids).Delete(&models.WebhookDelivery{}).Error
}

// Ping returns a test delivery for the webhook, which goes to it alone.
func Ping(hook *models.Webhook) (models.WebhookDelivery, error) {
	var data PingData

	data.Webhook.ID = hook.ID
	data.Webhook.Name = hook.Name
	data.Webhook.Events = append([]string{}, hook.Events...)

	eventID := uuid.NewString()

	payload, err := render(eventID, models.WebhookEventPing, data)
	if err != nil {
		return models.WebhookDelivery{}, err
	}

	return models.WebhookDelivery{
		WebhookID: hook.ID,
		EventID:   eventID,
		Event:     models.WebhookEventPing,
		Payload:   payload,
	}, nil
}

// Redelivery returns a new delivery of a delivery's event.
func Redelivery(of *models.WebhookDelivery) models.WebhookDelivery {
	return models.WebhookDelivery{
		WebhookID: of.WebhookID,
		EventID:   of.EventID,
		Event:     of.Event,
		Payload:   of.Payload,
	}
}

func render(eventID, event string, data any) (string, error) {
	body, err := json.Marshal(envelope{
		ID:        eventID,
		Event:     event,
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		Data:      data,
	})

	return string(body), err
}

// Translations loads the translations with the ids, for an event about them by the
// user. comment is a rejection's.
func Translations(tx *gorm.DB, ids []string, userID *string, comment string) (TranslationsData, error) {
	data := TranslationsData{Count: len(ids), Translations: []Translation{}, Comment: comment}
	ids, data.Truncated = capItems(ids)

	if err := tx.Table("localizations").
		Select("localizations.id, localizations.entry_id, entries.key, locales.code AS locale, localizations.translation").
		Joins("JOIN entries ON entries.id = localizations.entry_id").
		Joins("JOIN locales ON locales.id = localizations.locale_id").
		Where("localizations.id IN ?", ids).
		Order("entries.key, locales.code").
		Scan(&data.Translations).Error; err != nil {
		return data, err
	}

	var err error

	data.User, err = loadUser(tx, userID)

	return data, err
}

// Entries loads the entries with the ids, for an event about them by the user.
func Entries(tx *gorm.DB, ids []string, userID *string) (EntriesData, error) {
	data := EntriesData{Count: len(ids), Entries: []Entry{}}
	ids, data.Truncated = capItems(ids)

	if err := tx.Model(&models.Entry{}).
		Select("id, key, description").
		Where("id IN ?", ids).
		Order("key").
		Scan(&data.Entries).Error; err != nil {
		return data, err
	}

	var err error

	data.User, err = loadUser(tx, userID)

	return data, err
}

// TaskRun loads a run that ended, with its task, for an event about it.
func TaskRun(tx *gorm.DB, runID string) (TaskRunData, error) {
	var run models.TaskRun
	if err := tx.Preload("Task").First(&run, "id = ?", runID).Error; err != nil {
		return TaskRunData{}, err
	}

	user, err := loadUser(tx, run.TriggeredBy)
	if err != nil {
		return TaskRunData{}, err
	}

	data := TaskRunData{Run: RunInfo{
		ID:         run.ID,
		Trigger:    run.Trigger,
		Status:     run.Status,
		StartedAt:  utc(run.StartedAt),
		FinishedAt: utc(run.FinishedAt),
		Error:      run.Error,
		Summary:    run.RunSummary(),
		User:       user,
	}}

	if run.Task != nil {
		data.Task = TaskInfo{ID: run.Task.ID, Name: run.Task.Name, Type: run.Task.Type}
	}

	return data, nil
}

// capItems keeps the first maxItems ids, telling whether it dropped any.
func capItems(ids []string) ([]string, bool) {
	if len(ids) > maxItems {
		return ids[:maxItems], true
	}

	return ids, false
}

// loadUser returns who has the id, deleted users too, or nil without one.
func loadUser(tx *gorm.DB, id *string) (*User, error) {
	if id == nil || *id == "" {
		return nil, nil
	}

	var users []models.User
	if err := tx.Unscoped().Select("id", "name").Where("id = ?", *id).Limit(1).Find(&users).Error; err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, nil
	}

	return &User{ID: users[0].ID, Name: users[0].Name}, nil
}

func utc(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	converted := value.UTC()

	return &converted
}
