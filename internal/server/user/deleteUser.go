package user

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteUser soft deletes a user
func (s *Server) DeleteUser(
	ctx context.Context,
	req *connect.Request[pb.DeleteUserRequest],
) (*connect.Response[pb.DeleteUserResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteUser); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "user id is required")
	}

	var user models.User
	if err := s.db.First(&user, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrUserNotFound, "user not found")
	}

	if err := s.db.Delete(&user).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete user")
	}

	return connect.NewResponse(&pb.DeleteUserResponse{}), nil
}
