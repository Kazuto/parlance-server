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

func (s *Server) GetDefinition(
	ctx context.Context,
	req *connect.Request[pb.GetDefinitionRequest],
) (*connect.Response[pb.GetDefinitionResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadDefinition); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "definition id is required")
	}

	var definition models.Definition
	if err := s.db.First(&definition, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrDefinitionNotFound, "definition not found")
	}

	return connect.NewResponse(&pb.GetDefinitionResponse{
		Definition: converter.DefinitionToProto(&definition),
	}), nil
}
