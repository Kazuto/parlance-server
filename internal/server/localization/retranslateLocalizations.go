package localization

import (
	"context"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
)

// RetranslateLocalizations marks a language's translations for the machine
// translation tasks to translate again, or takes the marks back. A marked text stays,
// in exports too, until a task replaces it with a draft.
func (s *Server) RetranslateLocalizations(
	ctx context.Context,
	req *connect.Request[pb.RetranslateLocalizationsRequest],
) (*connect.Response[pb.RetranslateLocalizationsResponse], error) {
	if req.Msg.LocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	db := s.db.WithContext(ctx)

	locale, err := common.FindByID[models.Locale](db, req.Msg.LocaleId, errors.ErrLocaleNotFound, "locale not found")
	if err != nil {
		return nil, err
	}

	// The tasks translate from it, so its texts are never translated.
	if locale.IsDefault {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "the default language's texts are the ones translated")
	}

	var count, pending int64

	err = db.Transaction(func(tx *gorm.DB) error {
		// The language's translations of live entries in the scopes
		texts := func() *gorm.DB {
			entries := common.FilterEntriesByScopes(tx.Model(&models.Entry{}).Select("entries.id"), req.Msg.ScopeIds)

			return tx.Model(&models.Localization{}).Where("locale_id = ? AND entry_id IN (?)", locale.ID, entries)
		}

		selected := texts().Where("retranslate_at IS NULL")
		value := any(time.Now())

		switch {
		case req.Msg.Cancel:
			selected = texts().Where("retranslate_at IS NOT NULL")
			value = nil
		case !req.Msg.IncludeHuman:
			selected = selected.Where("origin = ?", models.OriginMachine)
		}

		if req.Msg.DryRun {
			if err := selected.Count(&count).Error; err != nil {
				return err
			}
		} else {
			// Not a change to the text: no history, and the update time stays.
			result := selected.UpdateColumn("retranslate_at", value)
			if result.Error != nil {
				return result.Error
			}

			count = result.RowsAffected
		}

		return texts().Where("retranslate_at IS NOT NULL").Count(&pending).Error
	})
	if err != nil {
		return nil, errors.InternalCause(err, "failed to mark translations")
	}

	return connect.NewResponse(&pb.RetranslateLocalizationsResponse{
		Count:   int32(count),
		Pending: int32(pending),
	}), nil
}
