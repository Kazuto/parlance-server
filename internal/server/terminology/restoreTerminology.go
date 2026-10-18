package terminology

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

func (s *Server) RestoreTerminology(
	ctx context.Context,
	req *connect.Request[pb.RestoreTerminologyRequest],
) (*connect.Response[pb.RestoreTerminologyResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteTerminology); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "terminology id is required")
	}

	var terminology models.Terminology
	if err := s.db.Unscoped().Preload("Definitions").First(&terminology, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrTerminologyNotFound, "terminology not found")
	}

	terminology.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&terminology).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to restore terminology")
	}

	return connect.NewResponse(&pb.RestoreTerminologyResponse{
		Terminology: converter.TerminologyToProto(&terminology),
	}), nil
}
