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

// RestoreUser restores a user by ID.
func (s *Server) RestoreUser(
	ctx context.Context,
	req *connect.Request[pb.RestoreUserRequest],
) (*connect.Response[pb.RestoreUserResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "user id is required")
	}

	user, err := common.Restore[models.User](ctx, s.db.Preload("Roles.Permissions"), req.Msg.Id, errors.ErrUserNotFound, "user not found")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.RestoreUserResponse{
		User: converter.UserToProto(&user),
	}), nil
}
