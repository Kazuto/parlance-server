package terminology

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) UpdateTerminology(
	ctx context.Context,
	req *connect.Request[pb.UpdateTerminologyRequest],
) (*connect.Response[pb.UpdateTerminologyResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("terminology id is required"))
	}
	if req.Msg.Term != nil && req.Msg.GetTerm() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("term is required"))
	}

	var terminology models.Terminology
	if err := s.db.First(&terminology, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("terminology not found"))
	}

	// Extract user ID from context
	userID, _ := ctx.Value("user_id").(string)
	if userID != "" {
		terminology.UpdatedBy = &userID
	}

	if req.Msg.Term != nil {
		terminology.Term = req.Msg.GetTerm()
	}

	if req.Msg.Description != nil {
		terminology.Description = req.Msg.GetDescription()
	}

	if err := s.db.Save(&terminology).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update terminology: %w", err))
	}

	// Reload with definitions
	if err := s.db.Preload("Definitions").First(&terminology, "id = ?", terminology.ID).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to reload terminology: %w", err))
	}

	return connect.NewResponse(&pb.UpdateTerminologyResponse{
		Terminology: converter.TerminologyToProto(&terminology),
	}), nil
}
