package release

import (
	"time"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/taskqueue"
	"github.com/kazuto/parlance-server/internal/webhook"
	"gorm.io/gorm"
)

// Publish lets the release out at now, by the user if someone did so by hand: webhooks
// hear of it, with the entries it let into exports, and tasks that run after releases
// queue. A release that went out already is RELEASE_ALREADY_RELEASED, so each goes out
// and is announced once. It runs in the caller's transaction.
func Publish(tx *gorm.DB, release *models.Release, now time.Time, userID *string) error {
	now = now.UTC()
	updates := map[string]any{"released_at": now, "updated_at": now}

	if userID != nil {
		updates["updated_by"] = *userID
	}

	result := tx.Model(&models.Release{}).Where("id = ? AND released_at IS NULL", release.ID).Updates(updates)
	if result.Error != nil {
		return errors.InternalCause(result.Error, "failed to release")
	}

	if result.RowsAffected == 0 {
		return errors.FailedPrecondition(errors.ErrReleaseAlreadyReleased, "the release went out already")
	}

	release.ReleasedAt = &now
	release.UpdatedAt = now

	if userID != nil {
		release.UpdatedBy = userID
	}

	if err := webhook.Emit(tx, models.WebhookEventTranslationsReleased, func() (any, error) {
		var ids []string

		// Its entries other releases still hold back don't export yet.
		if err := common.ExcludeHeldBack(tx.Model(&models.Entry{}), now).
			Where("entries.id IN (SELECT entry_id FROM release_entries WHERE release_id = ?)", release.ID).
			Order("entries.key").
			Pluck("entries.id", &ids).Error; err != nil {
			return nil, err
		}

		return webhook.Released(tx, release, ids, userID)
	}); err != nil {
		return errors.InternalCause(err, "failed to announce release")
	}

	if err := taskqueue.Emit(tx, models.EventTranslationsReleased); err != nil {
		return errors.InternalCause(err, "failed to queue tasks after release")
	}

	return nil
}
