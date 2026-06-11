package definition

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreDefinition(
	ctx context.Context,
	req *connect.Request[pb.RestoreDefinitionRequest],
) (*connect.Response[pb.RestoreDefinitionResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("definition id is required"))
	}

	var definition models.Definition
	if err := s.db.Unscoped().First(&definition, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("definition not found"))
	}

	definition.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&definition).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to restore definition: %w", err))
	}

	return connect.NewResponse(&pb.RestoreDefinitionResponse{
		Definition: converter.DefinitionToProto(&definition),
	}), nil
}
