package scope

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListScopes returns a paginated list of scopes
func (s *Server) ListScopes(
	ctx context.Context,
	req *connect.Request[pb.ListScopesRequest],
) (*connect.Response[pb.ListScopesResponse], error) {
	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var scopes []models.Scope
	var total int64

	query := s.db.Model(&models.Scope{})

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count scopes: %w", err))
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Order("name ASC").Find(&scopes).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch scopes: %w", err))
	}

	return connect.NewResponse(&pb.ListScopesResponse{
		Scopes:     converter.ScopesToProto(scopes),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
