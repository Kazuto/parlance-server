package role

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListRoles returns a paginated list of Roles
func (s *Server) ListRoles(
	ctx context.Context,
	req *connect.Request[pb.ListRolesRequest],
) (*connect.Response[pb.ListRolesResponse], error) {
	query, err := common.NewFilter(s.db.WithContext(ctx).Model(&models.Role{}), req.Msg.Filter).
		IncludeDeletedRequires(ctx, s.db, permissions.DeleteRole).
		AllowedSorts(map[string]bool{"name": true, "created_at": true, "updated_at": true}).
		DefaultSort("name ASC").
		Search(common.SearchColumns("name")).
		Apply()
	if err != nil {
		return nil, err
	}

	var roles []models.Role

	pagination, err := common.Paginate(query, req.Msg.Pagination, &roles, "Permissions")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ListRolesResponse{
		Roles:      converter.RolesToProto(roles),
		Pagination: pagination,
	}), nil
}
