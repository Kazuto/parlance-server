package scope

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListScopes returns a paginated list of scopes
func (s *Server) ListScopes(
	ctx context.Context,
	req *connect.Request[pb.ListScopesRequest],
) (*connect.Response[pb.ListScopesResponse], error) {
	query, err := common.NewFilter(s.db.WithContext(ctx).Model(&models.Scope{}), req.Msg.Filter).
		IncludeDeletedRequires(ctx, s.db, permissions.DeleteScope).
		AllowedSorts(map[string]bool{"name": true, "slug": true, "created_at": true, "updated_at": true}).
		DefaultSort("name ASC").
		Search(common.SearchColumns("name", "slug")).
		Apply()
	if err != nil {
		return nil, err
	}

	var scopes []models.Scope

	pagination, err := common.Paginate(query, req.Msg.Pagination, &scopes)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ListScopesResponse{
		Scopes:     converter.ScopesToProto(scopes),
		Pagination: pagination,
	}), nil
}
