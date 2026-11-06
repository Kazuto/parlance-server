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
	"github.com/kazuto/parlance-server/internal/webhook"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateLocalization creates a new localization
func (s *Server) CreateLocalization(
	ctx context.Context,
	req *connect.Request[pb.CreateLocalizationRequest],
) (*connect.Response[pb.CreateLocalizationResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	if req.Msg.LocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	if req.Msg.Translation == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "translation is required")
	}

	localization := &models.Localization{
		EntryID:     req.Msg.EntryId,
		LocaleID:    req.Msg.LocaleId,
		Translation: req.Msg.Translation,
		CreatedBy:   middleware.UserIDPtr(ctx),
		Review:      models.Review{Status: models.StatusDraft},
	}
	localization.WrittenBy(strings.TrimSpace(req.Msg.MachineModel))

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		at := time.Now()

		if err := tx.Create(localization).Error; err != nil {
			return err
		}

		// Translations written without a default text, or for a deleted one, wait for a
		// check against the new one.
		if err := outdateOthers(tx, localization, at); err != nil {
			return err
		}

		return webhook.EmitTranslations(tx, models.WebhookEventTranslationsAwaitingApproval, []string{localization.ID}, localization.CreatedBy, nil)
	}); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create localization")
	}

	return connect.NewResponse(&pb.CreateLocalizationResponse{
		Localization: converter.LocalizationToProto(localization),
	}), nil
}
