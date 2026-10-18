package definition

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) DeleteDefinition(
	ctx context.Context,
	req *connect.Request[pb.DeleteDefinitionRequest],
) (*connect.Response[pb.DeleteDefinitionResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteDefinition); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "definition id is required")
	}

	var definition models.Definition
	if err := s.db.First(&definition, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrDefinitionNotFound, "definition not found")
	}

	// Extract user ID from context for history tracking
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)
	if userID != "" {
		definition.UpdatedBy = &userID
	}

	if err := s.db.Delete(&definition).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete definition")
	}

	return connect.NewResponse(&pb.DeleteDefinitionResponse{}), nil
}
