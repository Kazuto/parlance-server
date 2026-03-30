package user

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateUser updates an existing user
func (s *Server) UpdateUser(
	ctx context.Context,
	req *connect.Request[pb.UpdateUserRequest],
) (*connect.Response[pb.UpdateUserResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user id is required"))
	}

	var user models.User
	if err := s.db.First(&user, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	if req.Msg.Email != "" {
		user.Email = req.Msg.Email
	}

	if req.Msg.Name != "" {
		user.Name = req.Msg.Name
	}

	if err := s.db.Save(&user).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update user: %w", err))
	}

	// Load roles for response
	s.db.Preload("Roles.Permissions").First(&user, "id = ?", user.ID)

	return connect.NewResponse(&pb.UpdateUserResponse{
		User: converter.UserToProto(&user),
	}), nil
}
