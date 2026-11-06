package common

import (
	"time"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

// MarkOutdated marks the translations of the entries in the other locales outdated, as
// their default text changed at the given time. Translations that are outdated already
// keep the time of the first change. Take the time before saving the default text, so
// its history row comes after it. Returns how many translations it marked.
//
// The text and review stay as they are, so it writes no history and runs no hooks.
func MarkOutdated(tx *gorm.DB, entryIDs []string, defaultLocaleID string, at time.Time) (int64, error) {
	if len(entryIDs) == 0 {
		return 0, nil
	}

	result := tx.Model(&models.Localization{}).
		Where("entry_id IN ? AND locale_id <> ? AND outdated_at IS NULL", UniqueIDs(entryIDs), defaultLocaleID).
		UpdateColumn("outdated_at", at)

	return result.RowsAffected, result.Error
}

// ClearAllOutdated marks every translation, deleted ones too, as fitting its default
// text, e.g. once another locale is the default.
func ClearAllOutdated(tx *gorm.DB) error {
	return tx.Unscoped().Model(&models.Localization{}).
		Where("outdated_at IS NOT NULL").
		UpdateColumn("outdated_at", nil).Error
}

// outdatable is a row that can fall behind its source text
type outdatable interface {
	ClearOutdated()
}
