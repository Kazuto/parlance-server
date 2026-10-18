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

// UpdateLocalization updates an existing localization
func (s *Server) UpdateLocalization(
	ctx context.Context,
	req *connect.Request[pb.UpdateLocalizationRequest],
) (*connect.Response[pb.UpdateLocalizationResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateEntry); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "localization id is required")
	}

	if req.Msg.Translation != nil && req.Msg.GetTranslation() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "translation is required")
	}

	var localization models.Localization
	if err := s.db.First(&localization, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocalizationNotFound, "localization not found")
	}

	// Extract user ID from context
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)
	if userID != "" {
		localization.UpdatedBy = &userID
	}

	if req.Msg.Translation != nil {
		localization.Translation = req.Msg.GetTranslation()
	}

	if err := s.db.Save(&localization).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update localization")
	}

	return connect.NewResponse(&pb.UpdateLocalizationResponse{
		Localization: converter.LocalizationToProto(&localization),
	}), nil
}
