package localization

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreLocalization(
	ctx context.Context,
	req *connect.Request[pb.RestoreLocalizationRequest],
) (*connect.Response[pb.RestoreLocalizationResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateEntry); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "localization id is required")
	}

	var localization models.Localization
	if err := s.db.Unscoped().First(&localization, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocalizationNotFound, "localization not found")
	}

	localization.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&localization).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to restore localization")
	}

	return connect.NewResponse(&pb.RestoreLocalizationResponse{
		Localization: converter.LocalizationToProto(&localization),
	}), nil
}
