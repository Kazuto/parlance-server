package user

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// RestoreUser restores a user by ID.
func (s *Server) RestoreUser(
	ctx context.Context,
	req *connect.Request[pb.RestoreUserRequest],
) (*connect.Response[pb.RestoreUserResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user id is required"))
	}

	var user models.User
	if err := s.db.Preload("Roles.Permissions").First(&user, "id = ?", req.Msg.Id).Unscoped().Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	user.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&user).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to restore user: %w", err))
	}

	return connect.NewResponse(&pb.RestoreUserResponse{
		User: converter.UserToProto(&user),
	}), nil
}
