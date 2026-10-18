package entry

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

// SearchEntries searches entries by query string
func (s *Server) SearchEntries(
	ctx context.Context,
	req *connect.Request[pb.SearchEntriesRequest],
) (*connect.Response[pb.SearchEntriesResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadEntry); err != nil {
		return nil, err
	}

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
		return nil, errors.Internal(errors.ErrInternal, "failed to count entries")
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Order("key ASC").Find(&entries).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to search entries")
	}

	return connect.NewResponse(&pb.SearchEntriesResponse{
		Entries:    converter.EntriesToProto(entries, "en"),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
