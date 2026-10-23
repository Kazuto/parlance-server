package user

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListUsers returns a paginated list of users
func (s *Server) ListUsers(
	ctx context.Context,
	req *connect.Request[pb.ListUsersRequest],
) (*connect.Response[pb.ListUsersResponse], error) {
	query, err := common.NewFilter(s.db.WithContext(ctx).Model(&models.User{}), req.Msg.Filter).
		IncludeDeletedRequires(ctx, s.db, permissions.DeleteUser).
		AllowedSorts(map[string]bool{"name": true, "email": true, "created_at": true, "updated_at": true}).
		DefaultSort("name ASC").
		Search(common.SearchColumns("name", "email")).
		Apply()
	if err != nil {
		return nil, err
	}

	var users []models.User

	pagination, err := common.Paginate(query, req.Msg.Pagination, &users, "Roles.Permissions")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ListUsersResponse{
		Users:      converter.UsersToProto(users),
		Pagination: pagination,
	}), nil
}
