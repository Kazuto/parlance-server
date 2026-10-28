package locale

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DestroyLocale permanently deletes a soft-deleted locale
func (s *Server) DestroyLocale(
	ctx context.Context,
	req *connect.Request[pb.DestroyLocaleRequest],
) (*connect.Response[pb.DestroyLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	locale, err := common.FindByID[models.Locale](s.db.WithContext(ctx).Unscoped(), req.Msg.Id, errors.ErrLocaleNotFound, "locale not found")
	if err != nil {
		return nil, err
	}

	// Only allow destroying already soft-deleted locales
	if locale.DeletedAt.Time.IsZero() {
		return nil, errors.FailedPrecondition(errors.ErrLocaleMustBeSoftDeleted, "locale must be soft-deleted before permanent deletion")
	}

	// Prevent destroying the default locale
	if locale.IsDefault {
		return nil, errors.FailedPrecondition(errors.ErrLocaleDefaultDestroy, "cannot destroy the default locale")
	}

	// Permanently delete the locale with its translations. History rows reference the purged
	// localizations and definitions, so they go first; the purged rows record no history.
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		purge := tx.Session(&gorm.Session{SkipHooks: true})
		for _, model := range []any{
			&models.LocalizationHistory{},
			&models.DefinitionHistory{},
			&models.Localization{},
			&models.Definition{},
		} {
			if err := purge.Unscoped().Where("locale_id = ?", locale.ID).Delete(model).Error; err != nil {
				return err
			}
		}

		return tx.Unscoped().Delete(&locale).Error
	})
	if err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to destroy locale")
	}

	return connect.NewResponse(&pb.DestroyLocaleResponse{}), nil
}
