package entry

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// SearchEntries searches entries by query string
func (s *Server) SearchEntries(
	ctx context.Context,
	req *connect.Request[pb.SearchEntriesRequest],
) (*connect.Response[pb.SearchEntriesResponse], error) {
	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var entries []models.Entry
	var total int64

	query := s.db.Model(&models.Entry{}).
		Preload("Localizations").
		Preload("Scopes")

	// Search in key and description
	if req.Msg.Query != "" {
		searchPattern := "%" + req.Msg.Query + "%"
		query = query.Where("key ILIKE ? OR description ILIKE ?", searchPattern, searchPattern)
	}

	// Filter by scope if provided
	if req.Msg.ScopeId != "" {
		query = query.Joins("JOIN entry_scopes ON entry_scopes.entry_id = entries.id").
			Where("entry_scopes.scope_id = ?", req.Msg.ScopeId)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count entries: %w", err))
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Order("key ASC").Find(&entries).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to search entries: %w", err))
	}

	return connect.NewResponse(&pb.SearchEntriesResponse{
		Entries:    converter.EntriesToProto(entries, "en"),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
