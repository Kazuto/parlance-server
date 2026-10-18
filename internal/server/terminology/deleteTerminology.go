package terminology

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) DeleteTerminology(
	ctx context.Context,
	req *connect.Request[pb.DeleteTerminologyRequest],
) (*connect.Response[pb.DeleteTerminologyResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteTerminology); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "terminology id is required")
	}

	var terminology models.Terminology
	if err := s.db.First(&terminology, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrTerminologyNotFound, "terminology not found")
	}

	// Extract user ID from context for history tracking
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)
	if userID != "" {
		terminology.UpdatedBy = &userID
	}

	if err := s.db.Delete(&terminology).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete terminology")
	}

	return connect.NewResponse(&pb.DeleteTerminologyResponse{}), nil
}
