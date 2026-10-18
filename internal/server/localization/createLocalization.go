package localization

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateLocalization creates a new localization
func (s *Server) CreateLocalization(
	ctx context.Context,
	req *connect.Request[pb.CreateLocalizationRequest],
) (*connect.Response[pb.CreateLocalizationResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateEntry); err != nil {
		return nil, err
	}

	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	if req.Msg.LocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	if req.Msg.Translation == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "translation is required")
	}

	// Extract user ID from context
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)

	localization := &models.Localization{
		EntryID:     req.Msg.EntryId,
		LocaleID:    req.Msg.LocaleId,
		Translation: req.Msg.Translation,
	}

	if userID != "" {
		localization.CreatedBy = &userID
	}

	if err := s.db.Create(localization).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create localization")
	}

	return connect.NewResponse(&pb.CreateLocalizationResponse{
		Localization: converter.LocalizationToProto(localization),
	}), nil
}
