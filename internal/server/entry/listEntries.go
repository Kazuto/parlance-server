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
	if req.Msg.TranslationStatus != "" && req.Msg.TranslationStatus != "missing" && req.Msg.TranslationStatus != "complete" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid translation status"))
	}
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

	// Filter by scopes if provided; an entry matches when it is in any of them.
	scopeIDs := req.Msg.ScopeIds
	if req.Msg.ScopeId != "" {
		scopeIDs = append(scopeIDs, req.Msg.ScopeId)
	}
	if len(scopeIDs) > 0 {
		query = filterScopes(query, scopeIDs)
	}

	// Filter by locale if provided
	if req.Msg.LocaleId != "" {
		query = query.Joins("JOIN localizations ON localizations.entry_id = entries.id").
			Where("localizations.locale_id = ?", req.Msg.LocaleId)
	}
	if req.Msg.TranslationStatus != "" {
		query = filterTranslationStatus(query, req.Msg.TranslationStatus, req.Msg.TargetLocaleIds)
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

// filterScopes keeps entries in any of the given scopes. A subquery instead of a
// join keeps entries that belong to several selected scopes from repeating.
func filterScopes(query *gorm.DB, scopeIDs []string) *gorm.DB {
	return query.Where("entries.id IN (SELECT entry_id FROM entry_scopes WHERE scope_id IN ?)", scopeIDs)
}

func filterTranslationStatus(query *gorm.DB, status string, localeIDs []string) *gorm.DB {
	// Start a separate statement so the correlated subquery doesn't inherit entry filters.
	targets := query.Session(&gorm.Session{NewDB: true}).Model(&models.Locale{}).Select("1")
	if len(localeIDs) > 0 {
		targets = targets.Where("locales.id IN ?", localeIDs)
	}
	targets = targets.Where("NOT EXISTS (SELECT 1 FROM localizations WHERE localizations.entry_id = entries.id AND localizations.locale_id = locales.id AND localizations.deleted_at IS NULL AND TRIM(localizations.translation) <> '')")
	if status == "missing" {
		return query.Where("EXISTS (?)", targets)
	}
	return query.Where("NOT EXISTS (?)", targets)
}
