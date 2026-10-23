package user

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteUser soft deletes a user
func (s *Server) DeleteUser(
	ctx context.Context,
	req *connect.Request[pb.DeleteUserRequest],
) (*connect.Response[pb.DeleteUserResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "user id is required")
	}

	user, err := common.FindByID[models.User](s.db.WithContext(ctx), req.Msg.Id, errors.ErrUserNotFound, "user not found")
	if err != nil {
		return nil, err
	}

	if err := common.DeleteRow(ctx, s.db.DB, &user); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to delete user")
	}

	return connect.NewResponse(&pb.DeleteUserResponse{}), nil
}
