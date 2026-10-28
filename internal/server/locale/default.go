package locale

import (
	"context"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
)

// saveLocale creates or updates the locale. A default locale replaces the previous one in the
// same transaction, so a failed save never leaves no default. Translations are compared
// with the default text, so a new default clears which ones are outdated.
func (s *Server) saveLocale(ctx context.Context, locale *models.Locale) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if locale.IsDefault {
			becomesDefault, err := isNewDefault(tx, locale)
			if err != nil {
				return err
			}

			others := tx.Model(&models.Locale{}).Where("is_default = ?", true)
			if locale.ID != "" {
				others = others.Where("id <> ?", locale.ID)
			}

			if err := others.Update("is_default", false).Error; err != nil {
				return err
			}

			if becomesDefault {
				if err := common.ClearAllOutdated(tx); err != nil {
					return err
				}
			}
		}

		return tx.Save(locale).Error
	})
	if err != nil {
		return errors.InternalCause(err, "failed to save locale")
	}

	return nil
}

// isNewDefault tells whether the default locale being saved wasn't the default before
func isNewDefault(tx *gorm.DB, locale *models.Locale) (bool, error) {
	if locale.ID == "" {
		return true, nil
	}

	var count int64

	err := tx.Model(&models.Locale{}).Where("id = ? AND is_default = ?", locale.ID, true).Count(&count).Error

	return count == 0, err
}
