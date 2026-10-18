package localization

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteLocalization soft deletes a localization
func (s *Server) DeleteLocalization(
	ctx context.Context,
	req *connect.Request[pb.DeleteLocalizationRequest],
) (*connect.Response[pb.DeleteLocalizationResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateEntry); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "localization id is required")
	}

	var localization models.Localization
	if err := s.db.First(&localization, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocalizationNotFound, "localization not found")
	}

	// Extract user ID from context for history tracking
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)
	if userID != "" {
		localization.UpdatedBy = &userID
	}

	if err := s.db.Delete(&localization).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete localization")
	}

	return connect.NewResponse(&pb.DeleteLocalizationResponse{}), nil
}
