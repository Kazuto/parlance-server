package terminology

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) DeleteTerminology(
	ctx context.Context,
	req *connect.Request[pb.DeleteTerminologyRequest],
) (*connect.Response[pb.DeleteTerminologyResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("terminology id is required"))
	}

	var terminology models.Terminology
	if err := s.db.First(&terminology, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("terminology not found"))
	}

	// Extract user ID from context for history tracking
	userID, _ := ctx.Value("user_id").(string)
	if userID != "" {
		terminology.UpdatedBy = &userID
	}

	if err := s.db.Delete(&terminology).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete terminology: %w", err))
	}

	return connect.NewResponse(&pb.DeleteTerminologyResponse{}), nil
}
