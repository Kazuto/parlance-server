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

// GetLocalization returns a single localization by ID
func (s *Server) GetLocalization(
	ctx context.Context,
	req *connect.Request[pb.GetLocalizationRequest],
) (*connect.Response[pb.GetLocalizationResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadEntry); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "localization id is required")
	}

	var localization models.Localization
	if err := s.db.First(&localization, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocalizationNotFound, "localization not found")
	}

	return connect.NewResponse(&pb.GetLocalizationResponse{
		Localization: converter.LocalizationToProto(&localization),
	}), nil
}
