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

// ListUsers returns a paginated list of users
func (s *Server) ListUsers(
	ctx context.Context,
	req *connect.Request[pb.ListUsersRequest],
) (*connect.Response[pb.ListUsersResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadUser); err != nil {
		return nil, err
	}

	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var users []models.User
	var total int64

	query := s.db.Model(&models.User{})

	if err := query.Count(&total).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to count users")
	}

	offset := (page - 1) * perPage
	if err := query.Preload("Roles.Permissions").Offset(int(offset)).Limit(int(perPage)).Order("name ASC").Find(&users).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to fetch users")
	}

	usersProto := make([]*pb.User, len(users))
	for i, user := range users {
		usersProto[i] = converter.UserToProto(&user)
	}

	return connect.NewResponse(&pb.ListUsersResponse{
		Users:      usersProto,
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
