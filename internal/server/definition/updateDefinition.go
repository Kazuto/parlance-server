package definition

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

func (s *Server) UpdateDefinition(
	ctx context.Context,
	req *connect.Request[pb.UpdateDefinitionRequest],
) (*connect.Response[pb.UpdateDefinitionResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateDefinition); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "definition id is required")
	}

	if req.Msg.Translation != nil && req.Msg.GetTranslation() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "translation is required")
	}

	var definition models.Definition
	if err := s.db.First(&definition, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrDefinitionNotFound, "definition not found")
	}

	// Extract user ID from context
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)
	if userID != "" {
		definition.UpdatedBy = &userID
	}

	if req.Msg.Translation != nil {
		definition.Translation = req.Msg.GetTranslation()
	}

	if err := s.db.Save(&definition).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update definition")
	}

	return connect.NewResponse(&pb.UpdateDefinitionResponse{
		Definition: converter.DefinitionToProto(&definition),
	}), nil
}
