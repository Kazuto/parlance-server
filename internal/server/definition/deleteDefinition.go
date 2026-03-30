package definition

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) DeleteDefinition(
	ctx context.Context,
	req *connect.Request[pb.DeleteDefinitionRequest],
) (*connect.Response[pb.DeleteDefinitionResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("definition id is required"))
	}

	var definition models.Definition
	if err := s.db.First(&definition, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("definition not found"))
	}

	// Extract user ID from context for history tracking
	userID, _ := ctx.Value("user_id").(string)
	if userID != "" {
		definition.UpdatedBy = &userID
	}

	if err := s.db.Delete(&definition).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete definition: %w", err))
	}

	return connect.NewResponse(&pb.DeleteDefinitionResponse{}), nil
}
