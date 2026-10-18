package user

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// RestoreUser restores a user by ID.
func (s *Server) RestoreUser(
	ctx context.Context,
	req *connect.Request[pb.RestoreUserRequest],
) (*connect.Response[pb.RestoreUserResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteUser); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "user id is required")
	}

	var user models.User
	if err := s.db.Unscoped().Preload("Roles.Permissions").First(&user, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrUserNotFound, "user not found")
	}

	user.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&user).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to restore user")
	}

	return connect.NewResponse(&pb.RestoreUserResponse{
		User: converter.UserToProto(&user),
	}), nil
}
