package definition

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

func (s *Server) RestoreDefinition(
	ctx context.Context,
	req *connect.Request[pb.RestoreDefinitionRequest],
) (*connect.Response[pb.RestoreDefinitionResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteDefinition); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "definition id is required")
	}

	var definition models.Definition
	if err := s.db.Unscoped().First(&definition, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrDefinitionNotFound, "definition not found")
	}

	definition.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&definition).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to restore definition")
	}

	return connect.NewResponse(&pb.RestoreDefinitionResponse{
		Definition: converter.DefinitionToProto(&definition),
	}), nil
}
