package scope

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreScope(
	ctx context.Context,
	req *connect.Request[pb.RestoreScopeRequest],
) (*connect.Response[pb.RestoreScopeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope id is required"))
	}

	var scope models.Scope
	if err := s.db.Unscoped().First(&scope, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scope not found"))
	}

	scope.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&scope).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to restore scope: %w", err))
	}

	return connect.NewResponse(&pb.RestoreScopeResponse{
		Scope: converter.ScopeToProto(&scope),
	}), nil
}
