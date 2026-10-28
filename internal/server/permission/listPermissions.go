package permission

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListPermissions returns a paginated list of Permissions
func (s *Server) ListPermissions(
	ctx context.Context,
	req *connect.Request[pb.ListPermissionsRequest],
) (*connect.Response[pb.ListPermissionsResponse], error) {
	query, err := common.NewFilter(s.db.WithContext(ctx).Model(&models.Permission{}), req.Msg.Filter).
		AllowedSorts(map[string]bool{"name": true, "resource": true, "action": true, "created_at": true, "updated_at": true}).
		DefaultSort("name ASC").
		Search(common.SearchColumns("name", "resource")).
		Apply()
	if err != nil {
		return nil, err
	}

	var permissions []models.Permission

	pagination, err := common.Paginate(query, req.Msg.Pagination, &permissions)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ListPermissionsResponse{
		Permissions: converter.PermissionsToProto(permissions),
		Pagination:  pagination,
	}), nil
}
