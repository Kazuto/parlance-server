package user

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetUser returns a single user by ID
func (s *Server) GetUser(
	ctx context.Context,
	req *connect.Request[pb.GetUserRequest],
) (*connect.Response[pb.GetUserResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "user id is required")
	}

	user, err := common.FindByID[models.User](s.db.WithContext(ctx).Preload("Roles.Permissions"), req.Msg.Id, errors.ErrUserNotFound, "user not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetUserResponse{
		User: converter.UserToProto(&user),
	}), nil
}
