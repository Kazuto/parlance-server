package entry

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListEntries returns a paginated list of entries
func (s *Server) ListEntries(
	ctx context.Context,
	req *connect.Request[pb.ListEntriesRequest],
) (*connect.Response[pb.ListEntriesResponse], error) {
	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var entries []models.Entry
	var total int64

	query := s.db.Model(&models.Entry{}).
		Preload("Localizations").
		Preload("Scopes")

	allowedSorts := map[string]bool{"key": true, "description": true, "created_at": true, "created_by": true, "updated_at": true, "updated_by": true}
	query, err := common.NewFilter(query, req.Msg.Filter).
		AllowedSorts(allowedSorts).
		DefaultSort("key ASC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return query.Where("key LIKE ?", "%"+search+"%")
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	// Filter by scope if provided
	if req.Msg.ScopeId != "" {
		query = query.Joins("JOIN entry_scopes ON entry_scopes.entry_id = entries.id").
			Where("entry_scopes.scope_id = ?", req.Msg.ScopeId)
	}

	// Filter by locale if provided
	if req.Msg.LocaleId != "" {
		query = query.Joins("JOIN localizations ON localizations.entry_id = entries.id").
			Where("localizations.locale_id = ?", req.Msg.LocaleId)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count entries: %w", err))
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Find(&entries).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch entries: %w", err))
	}

	return connect.NewResponse(&pb.ListEntriesResponse{
		Entries:    converter.EntriesToProto(entries, "en"),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
