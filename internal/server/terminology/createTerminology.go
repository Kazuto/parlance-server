package terminology

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) CreateTerminology(
	ctx context.Context,
	req *connect.Request[pb.CreateTerminologyRequest],
) (*connect.Response[pb.CreateTerminologyResponse], error) {
	if req.Msg.Term == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("term is required"))
	}

	// Extract user ID from context
	userID, _ := ctx.Value("user_id").(string)

	terminology := &models.Terminology{
		Term:        req.Msg.Term,
		Description: req.Msg.Description,
	}

	if userID != "" {
		terminology.CreatedBy = &userID
	}

	if err := s.db.Create(terminology).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create terminology: %w", err))
	}

	return connect.NewResponse(&pb.CreateTerminologyResponse{
		Terminology: converter.TerminologyToProto(terminology),
	}), nil
}
