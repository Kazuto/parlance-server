package localization

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/webhook"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateLocalization updates an existing localization
func (s *Server) UpdateLocalization(
	ctx context.Context,
	req *connect.Request[pb.UpdateLocalizationRequest],
) (*connect.Response[pb.UpdateLocalizationResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "localization id is required")
	}

	localization, err := common.FindByID[models.Localization](s.db.WithContext(ctx), req.Msg.Id, errors.ErrLocalizationNotFound, "localization not found")
	if err != nil {
		return nil, err
	}

	// Credit the change, including a deletion, to the current user
	localization.UpdatedBy = middleware.UserIDPtr(ctx)

	// If translation is being set to empty, delete the localization instead
	if req.Msg.Translation != nil && req.Msg.GetTranslation() == "" {
		if err := s.db.Delete(&localization).Error; err != nil {
			return nil, errors.Internal(errors.ErrInternal, "failed to delete localization")
		}

		return connect.NewResponse(&pb.UpdateLocalizationResponse{
			Localization: nil,
		}), nil
	}

	// New text needs review again, and is written for the default text as it is now
	changed := req.Msg.Translation != nil && req.Msg.GetTranslation() != localization.Translation
	if changed {
		localization.Translation = req.Msg.GetTranslation()
		localization.MarkDraft()
		localization.ClearOutdated()
		localization.WrittenBy(strings.TrimSpace(req.Msg.MachineModel))
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		at := time.Now()

		if err := tx.Save(&localization).Error; err != nil {
			return err
		}

		if !changed {
			return nil
		}

		if err := outdateOthers(tx, &localization, at); err != nil {
			return err
		}

		return webhook.EmitTranslations(tx, models.WebhookEventTranslationsAwaitingApproval, []string{localization.ID}, localization.UpdatedBy, nil)
	}); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update localization")
	}

	return connect.NewResponse(&pb.UpdateLocalizationResponse{
		Localization: converter.LocalizationToProto(&localization),
	}), nil
}

// outdateOthers marks the entry's other translations outdated when the localization is
// the default locale's text, which changed at the given time.
func outdateOthers(tx *gorm.DB, localization *models.Localization, at time.Time) error {
	defaultLocale, err := common.DefaultLocale(tx)
	if err != nil || defaultLocale == nil || defaultLocale.ID != localization.LocaleID {
		return err
	}

	_, err = common.MarkOutdated(tx, []string{localization.EntryID}, defaultLocale.ID, at)

	return err
}
