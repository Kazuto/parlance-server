package locale

import (
	"context"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

// saveLocale creates or updates the locale. A default locale replaces the previous one in the
// same transaction, so a failed save never leaves no default.
func (s *Server) saveLocale(ctx context.Context, locale *models.Locale) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if locale.IsDefault {
			others := tx.Model(&models.Locale{}).Where("is_default = ?", true)
			if locale.ID != "" {
				others = others.Where("id <> ?", locale.ID)
			}

			if err := others.Update("is_default", false).Error; err != nil {
				return err
			}
		}

		return tx.Save(locale).Error
	})
	if err != nil {
		return errors.InternalCause(err, "failed to save locale")
	}

	return nil
}
