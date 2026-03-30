package user

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetUser returns a single user by ID
func (s *Server) GetUser(
	ctx context.Context,
	req *connect.Request[pb.GetUserRequest],
) (*connect.Response[pb.GetUserResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("user id is required"))
	}

	var user models.User
	if err := s.db.Preload("Roles.Permissions").First(&user, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	return connect.NewResponse(&pb.GetUserResponse{
		User: converter.UserToProto(&user),
	}), nil
}
