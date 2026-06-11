package terminology

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreTerminology(
	ctx context.Context,
	req *connect.Request[pb.RestoreTerminologyRequest],
) (*connect.Response[pb.RestoreTerminologyResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("terminology id is required"))
	}

	var terminology models.Terminology
	if err := s.db.Preload("Definitions").First(&terminology, "id = ?", req.Msg.Id).Unscoped().Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("terminology not found"))
	}

	terminology.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&terminology).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to restore terminology: %w", err))
	}

	return connect.NewResponse(&pb.RestoreTerminologyResponse{
		Terminology: converter.TerminologyToProto(&terminology),
	}), nil
}
