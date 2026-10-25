package entry

import (
	"context"
	"encoding/base64"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
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
	if !slices.Contains([]string{"", "missing", "complete", "review", "outdated"}, req.Msg.TranslationStatus) {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "invalid translation status")
	}

	// Deleted entries are listed to restore them, which takes the same permission.
	if req.Msg.DeletedOnly {
		if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteEntry); err != nil {
			return nil, err
		}
	}

	keyOrdered := req.Msg.Filter == nil || req.Msg.Filter.Sort == "" ||
		(strings.EqualFold(req.Msg.Filter.Sort, "key") && !strings.EqualFold(req.Msg.Filter.Order, "desc"))

	var afterKey string

	if req.Msg.Cursor != "" {
		if !keyOrdered {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "cursor requires key ordering")
		}

		key, err := decodeCursor(req.Msg.Cursor)
		if err != nil {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "invalid cursor")
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

	allowedSorts := map[string]bool{"key": true, "description": true, "created_at": true, "created_by": true, "updated_at": true, "updated_by": true, "deleted_at": true}

	query, err := common.NewFilter(query, req.Msg.Filter).
		IncludeDeletedRequires(ctx, s.db, permissions.DeleteEntry).
		AllowedSorts(allowedSorts).
		DefaultSort("key ASC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return searchEntries(query, search, req.Msg.TargetLocaleIds)
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	if req.Msg.DeletedOnly {
		query = query.Unscoped().Where("entries.deleted_at IS NOT NULL")
	}

	// Filter by scopes if provided; an entry matches when it is in any of them.
	scopeIDs := req.Msg.ScopeIds
	if req.Msg.ScopeId != "" {
		scopeIDs = append(scopeIDs, req.Msg.ScopeId)
	}

	if len(scopeIDs) > 0 {
		query = common.FilterEntriesByScopes(query, scopeIDs)
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
			return nil, errors.Internal(errors.ErrInternal, "failed to count entries")
		}

		pagination = converter.PaginationToProto(total, page, perPage)
		query = query.Offset(int((page - 1) * perPage))
	} else {
		query = query.Where("entries.key > ?", afterKey)
	}

	// One extra row tells whether another page follows without a second count.
	if err := query.Limit(int(perPage) + 1).Find(&entries).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to fetch entries")
	}

	nextCursor := ""

	if len(entries) > int(perPage) {
		entries = entries[:perPage]
		if keyOrdered {
			nextCursor = encodeCursor(entries[len(entries)-1].Key)
		}
	}

	result := converter.EntriesToProto(entries)
	if req.Msg.Summary {
		if err := s.fillMissingLocales(result, req.Msg.TargetLocaleIds); err != nil {
			return nil, err
		}

		if err := s.fillDefaultTranslations(result); err != nil {
			return nil, err
		}
	}

	return connect.NewResponse(&pb.ListEntriesResponse{
		Entries:    result,
		Pagination: pagination,
		NextCursor: nextCursor,
	}), nil
}

// maxDefaultTranslationLength caps default_translation in runes: it is a preview,
// and a page of long texts would otherwise bloat the summary listing.
const maxDefaultTranslationLength = 200

// fillDefaultTranslations sets default_translation on each entry: its text in the
// default locale, shortened to maxDefaultTranslationLength runes.
func (s *Server) fillDefaultTranslations(entries []*pb.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	defaultLocale, err := common.DefaultLocale(s.db.DB)
	if err != nil {
		return err
	}

	if defaultLocale == nil {
		return nil
	}

	entryIDs := make([]string, len(entries))
	for i, entry := range entries {
		entryIDs[i] = entry.Id
	}

	var rows []struct {
		EntryID     string
		Translation string
	}

	err = s.db.Model(&models.Localization{}).
		Select("entry_id, translation").
		Where("entry_id IN ? AND locale_id = ?", entryIDs, defaultLocale.ID).
		Scan(&rows).Error
	if err != nil {
		return errors.Internal(errors.ErrInternal, "failed to load default translations")
	}

	translations := make(map[string]string, len(rows))
	for _, row := range rows {
		translations[row.EntryID] = shorten(strings.TrimSpace(row.Translation), maxDefaultTranslationLength)
	}

	for _, entry := range entries {
		entry.DefaultTranslation = translations[entry.Id]
	}

	return nil
}

// shorten cuts text to at most limit runes, ending in an ellipsis when cut.
func shorten(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}

	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}

func encodeCursor(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

func decodeCursor(cursor string) (string, error) {
	key, err := base64.RawURLEncoding.DecodeString(cursor)

	return string(key), err
}

// fillMissingLocales sets missing_locale_ids on each entry: the target locales (all
// locales when none are given) without a saved, non-blank translation,
// draft_locale_ids: those whose translation awaits review, and outdated_locale_ids:
// those whose translation is outdated.
func (s *Server) fillMissingLocales(entries []*pb.Entry, targetLocaleIDs []string) error {
	if len(entries) == 0 {
		return nil
	}

	targets := targetLocaleIDs
	if len(targets) == 0 {
		if err := s.db.Model(&models.Locale{}).Pluck("id", &targets).Error; err != nil {
			return errors.Internal(errors.ErrInternal, "failed to load locales")
		}
	}

	entryIDs := make([]string, len(entries))
	for i, entry := range entries {
		entryIDs[i] = entry.Id
	}

	var present []struct {
		EntryID  string
		LocaleID string
		Status   string
		Outdated bool
	}

	if len(targets) > 0 {
		err := s.db.Model(&models.Localization{}).
			Select("entry_id, locale_id, status, outdated_at IS NOT NULL AS outdated").
			Where("entry_id IN ? AND locale_id IN ? AND TRIM(translation) <> ''", entryIDs, targets).
			Scan(&present).Error
		if err != nil {
			return errors.Internal(errors.ErrInternal, "failed to load translations")
		}
	}

	// entry id -> locale id -> state of the translation
	translated := make(map[string]map[string]translationState, len(entries))
	for _, row := range present {
		if translated[row.EntryID] == nil {
			translated[row.EntryID] = map[string]translationState{}
		}

		translated[row.EntryID][row.LocaleID] = translationState{Status: row.Status, Outdated: row.Outdated}
	}

	for _, entry := range entries {
		missing := make([]string, 0, len(targets))
		drafts := make([]string, 0)
		outdated := make([]string, 0)

		for _, localeID := range targets {
			state, ok := translated[entry.Id][localeID]
			if !ok {
				missing = append(missing, localeID)

				continue
			}

			if state.Status == models.StatusDraft {
				drafts = append(drafts, localeID)
			}

			if state.Outdated {
				outdated = append(outdated, localeID)
			}
		}

		entry.MissingLocaleIds = missing
		entry.DraftLocaleIds = drafts
		entry.OutdatedLocaleIds = outdated
	}

	return nil
}

// translationState is where a translation stands: its review status, and whether its
// default text changed since
type translationState struct {
	Status   string
	Outdated bool
}

// searchEntries keeps entries whose key, or whose translation in one of the given
// locales (any locale when none are given), contains search, ignoring case. The
// locales are the ones the editor shows, so a match is visible once opened.
func searchEntries(query *gorm.DB, search string, localeIDs []string) *gorm.DB {
	pattern := "%" + strings.ToLower(search) + "%"
	// Start a separate statement so the correlated subquery doesn't inherit entry filters.
	translations := query.Session(&gorm.Session{NewDB: true}).Table("localizations").Select("1").
		Where("localizations.entry_id = entries.id AND localizations.deleted_at IS NULL AND LOWER(localizations.translation) LIKE ?", pattern)
	if len(localeIDs) > 0 {
		translations = translations.Where("localizations.locale_id IN ?", localeIDs)
	}

	return query.Where("(LOWER(entries.key) LIKE ? OR EXISTS (?))", pattern, translations)
}

// filterTranslationStatus keeps entries by their translations in the given locales (all
// when none are given): "missing" one, "complete" in all, one in "review" as a draft, or
// one "outdated".
func filterTranslationStatus(query *gorm.DB, status string, localeIDs []string) *gorm.DB {
	// Start a separate statement so the correlated subquery doesn't inherit entry filters.
	targets := query.Session(&gorm.Session{NewDB: true}).Model(&models.Locale{}).Select("1")
	if len(localeIDs) > 0 {
		targets = targets.Where("locales.id IN ?", localeIDs)
	}

	// The entry's non-blank translation in the locale
	const translation = "SELECT 1 FROM localizations WHERE localizations.entry_id = entries.id AND localizations.locale_id = locales.id AND localizations.deleted_at IS NULL AND TRIM(localizations.translation) <> ''"

	switch status {
	case "review":
		return query.Where("EXISTS (?)", targets.Where("EXISTS ("+translation+" AND localizations.status = ?)", models.StatusDraft))
	case "outdated":
		return query.Where("EXISTS (?)", targets.Where("EXISTS ("+translation+" AND localizations.outdated_at IS NOT NULL)"))
	}

	targets = targets.Where("NOT EXISTS (" + translation + ")")
	if status == "missing" {
		return query.Where("EXISTS (?)", targets)
	}

	return query.Where("NOT EXISTS (?)", targets)
}
