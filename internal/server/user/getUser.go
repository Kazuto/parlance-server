package user

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetUser returns a single user by ID
func (s *Server) GetUser(
	ctx context.Context,
	req *connect.Request[pb.GetUserRequest],
) (*connect.Response[pb.GetUserResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadUser); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "user id is required")
	}

	var user models.User
	if err := s.db.Preload("Roles.Permissions").First(&user, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrUserNotFound, "user not found")
	}

	return connect.NewResponse(&pb.GetUserResponse{
		User: converter.UserToProto(&user),
	}), nil
}
