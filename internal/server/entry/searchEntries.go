package entry

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// SearchEntries searches entries by query string
func (s *Server) SearchEntries(
	ctx context.Context,
	req *connect.Request[pb.SearchEntriesRequest],
) (*connect.Response[pb.SearchEntriesResponse], error) {
	query := s.db.WithContext(ctx).Model(&models.Entry{}).Order("key ASC")
	if req.Msg.Query != "" {
		query = common.SearchColumns("key", "description")(query, req.Msg.Query)
	}

	if req.Msg.ScopeId != "" {
		query = common.FilterEntriesByScopes(query, []string{req.Msg.ScopeId})
	}

	var entries []models.Entry

	pagination, err := common.Paginate(query, req.Msg.Pagination, &entries, "Localizations", "Scopes")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.SearchEntriesResponse{
		Entries:    converter.EntriesToProto(entries),
		Pagination: pagination,
	}), nil
}
