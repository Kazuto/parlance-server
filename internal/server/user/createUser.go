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

// CreateUser creates a new user
func (s *Server) CreateUser(
	ctx context.Context,
	req *connect.Request[pb.CreateUserRequest],
) (*connect.Response[pb.CreateUserResponse], error) {
	var roles []models.Role

	if ids := req.Msg.GetRoles().GetIds(); len(ids) > 0 {
		var err error

		roles, err = common.ResolveByIDs[models.Role](s.db.WithContext(ctx), ids, errors.ErrRoleNotFound, "one or more roles do not exist")
		if err != nil {
			return nil, err
		}
	}

	user, err := common.CreateUser(ctx, s.db.DB, common.NewUser{
		Email:       req.Msg.Email,
		Name:        req.Msg.Name,
		Password:    req.Msg.Password,
		Roles:       roles,
		DefaultRole: "viewer",
	})
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.CreateUserResponse{
		User: converter.UserToProto(user),
	}), nil
}
