package entry

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListEntries returns a paginated list of entries.
//
// Two paging modes share one endpoint: page/per_page with a total count, or keyset
// paging via cursor, which stays fast at any depth and doesn't skip or repeat rows
// when entries are added while paging. Both return next_cursor in key order.
func (s *Server) ListEntries(
	ctx context.Context,
	req *connect.Request[pb.ListEntriesRequest],
) (*connect.Response[pb.ListEntriesResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadEntry); err != nil {
		return nil, err
	}

	if req.Msg.TranslationStatus != "" && req.Msg.TranslationStatus != "missing" && req.Msg.TranslationStatus != "complete" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid translation status"))
	}
	keyOrdered := req.Msg.Filter == nil || req.Msg.Filter.Sort == "" ||
		(strings.EqualFold(req.Msg.Filter.Sort, "key") && !strings.EqualFold(req.Msg.Filter.Order, "desc"))
	var afterKey string
	if req.Msg.Cursor != "" {
		if !keyOrdered {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("cursor requires key ordering"))
		}
		key, err := decodeCursor(req.Msg.Cursor)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid cursor"))
		}
		afterKey = key
	}
	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var entries []models.Entry
	var total int64

	query := s.db.Model(&models.Entry{})
	if !req.Msg.Summary {
		query = query.Preload("Localizations").Preload("Scopes")
	}

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

	var pagination *pb.PaginationResponse
	if req.Msg.Cursor == "" {
		if err := query.Count(&total).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count entries: %w", err))
		}
		pagination = converter.PaginationToProto(total, page, perPage)
		query = query.Offset(int((page - 1) * perPage))
	} else {
		query = query.Where("entries.key > ?", afterKey)
	}
	if req.Msg.Filter == nil {
		query = query.Order("key ASC")
	}

	// One extra row tells whether another page follows without a second count.
	if err := query.Limit(int(perPage) + 1).Find(&entries).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch entries: %w", err))
	}
	nextCursor := ""
	if len(entries) > int(perPage) {
		entries = entries[:perPage]
		if keyOrdered {
			nextCursor = encodeCursor(entries[len(entries)-1].Key)
		}
	}

	result := converter.EntriesToProto(entries, "en")
	if req.Msg.Summary {
		if err := s.fillMissingLocales(result, req.Msg.TargetLocaleIds); err != nil {
			return nil, err
		}
	}

	return connect.NewResponse(&pb.ListEntriesResponse{
		Entries:    result,
		Pagination: pagination,
		NextCursor: nextCursor,
	}), nil
}

func encodeCursor(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

func decodeCursor(cursor string) (string, error) {
	key, err := base64.RawURLEncoding.DecodeString(cursor)
	return string(key), err
}

// fillMissingLocales sets missing_locale_ids on each entry: the target locales (all
// locales when none are given) without a saved, non-blank translation.
func (s *Server) fillMissingLocales(entries []*pb.Entry, targetLocaleIDs []string) error {
	if len(entries) == 0 {
		return nil
	}
	targets := targetLocaleIDs
	if len(targets) == 0 {
		if err := s.db.Model(&models.Locale{}).Pluck("id", &targets).Error; err != nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load locales: %w", err))
		}
	}
	entryIDs := make([]string, len(entries))
	for i, entry := range entries {
		entryIDs[i] = entry.Id
	}
	var present []struct {
		EntryID  string
		LocaleID string
	}
	if len(targets) > 0 {
		err := s.db.Model(&models.Localization{}).
			Select("entry_id, locale_id").
			Where("entry_id IN ? AND locale_id IN ? AND TRIM(translation) <> ''", entryIDs, targets).
			Scan(&present).Error
		if err != nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load translations: %w", err))
		}
	}
	translated := make(map[string]map[string]bool, len(entries))
	for _, row := range present {
		if translated[row.EntryID] == nil {
			translated[row.EntryID] = map[string]bool{}
		}
		translated[row.EntryID][row.LocaleID] = true
	}
	for _, entry := range entries {
		missing := make([]string, 0, len(targets))
		for _, localeID := range targets {
			if !translated[entry.Id][localeID] {
				missing = append(missing, localeID)
			}
		}
		entry.MissingLocaleIds = missing
	}
	return nil
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
