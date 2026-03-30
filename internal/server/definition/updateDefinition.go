package definition

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) UpdateDefinition(
	ctx context.Context,
	req *connect.Request[pb.UpdateDefinitionRequest],
) (*connect.Response[pb.UpdateDefinitionResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("definition id is required"))
	}

	if req.Msg.Translation == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("translation is required"))
	}

	var definition models.Definition
	if err := s.db.First(&definition, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("definition not found"))
	}

	// Extract user ID from context
	userID, _ := ctx.Value("user_id").(string)
	if userID != "" {
		definition.UpdatedBy = &userID
	}

	definition.Translation = req.Msg.Translation

	if err := s.db.Save(&definition).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update definition: %w", err))
	}

	return connect.NewResponse(&pb.UpdateDefinitionResponse{
		Definition: converter.DefinitionToProto(&definition),
	}), nil
}
