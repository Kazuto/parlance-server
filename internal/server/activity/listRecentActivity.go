package activity

import (
	"context"
	"fmt"
	"sort"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

const (
	defaultLimit = 10
	maxLimit     = 50
	// Rows read per history table. A burst reaching past this window (e.g. a huge import) is
	// marked partial, as its counts only cover the rows read.
	scanSize = 1000
)

type historyRow struct {
	ID        string
	Action    string
	UserID    *string
	LocaleID  string
	SubjectID string
	ChangedAt time.Time
}

// ListRecentActivity returns recent translation and glossary changes, grouped into bursts.
func (s *Server) ListRecentActivity(
	ctx context.Context,
	req *connect.Request[pb.ListRecentActivityRequest],
) (*connect.Response[pb.ListRecentActivityResponse], error) {
	limit := int(req.Msg.Limit)
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)

	userID := ""
	if req.Msg.OnlyMine {
		userID, _ = ctx.Value(middleware.ContextKeyUserID).(string)
		if userID == "" {
			return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("user not authenticated"))
		}
	}

	db := s.db.WithContext(ctx)
	translations, err := recentRows(db, "localization_history", "entry_id", userID)
	if err != nil {
		return nil, err
	}
	definitions, err := recentRows(db, "definition_history", "terminology_id", userID)
	if err != nil {
		return nil, err
	}

	changes := make([]change, 0, len(translations)+len(definitions))
	changes = appendChanges(changes, translations, pb.ActivityKind_ACTIVITY_KIND_TRANSLATION)
	changes = appendChanges(changes, definitions, pb.ActivityKind_ACTIVITY_KIND_DEFINITION)
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].ChangedAt.After(changes[j].ChangedAt) })

	// Where a history table was cut off: groups reaching back that far may be incomplete.
	edges := map[pb.ActivityKind]time.Time{}
	for kind, rows := range map[pb.ActivityKind][]historyRow{
		pb.ActivityKind_ACTIVITY_KIND_TRANSLATION: translations,
		pb.ActivityKind_ACTIVITY_KIND_DEFINITION:  definitions,
	} {
		if len(rows) == scanSize {
			edges[kind] = rows[len(rows)-1].ChangedAt
		}
	}

	groups := groupChanges(changes, limit)
	items, err := s.toItems(db, groups, edges, common.GetLocaleFromRequest(req))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ListRecentActivityResponse{Items: items}), nil
}

func recentRows(db *gorm.DB, table, subjectColumn, userID string) ([]historyRow, error) {
	var rows []historyRow
	query := db.Table(table).
		Select("id, action, user_id, locale_id, " + subjectColumn + " AS subject_id, changed_at").
		Order("changed_at DESC").
		Limit(scanSize)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load %s: %w", table, err))
	}
	return rows, nil
}

func appendChanges(changes []change, rows []historyRow, kind pb.ActivityKind) []change {
	for _, row := range rows {
		userID := ""
		if row.UserID != nil {
			userID = *row.UserID
		}
		changes = append(changes, change{
			ID:        row.ID,
			Kind:      kind,
			Action:    row.Action,
			UserID:    userID,
			LocaleID:  row.LocaleID,
			SubjectID: row.SubjectID,
			ChangedAt: row.ChangedAt,
		})
	}
	return changes
}

// toItems resolves user names, locale names and entry keys / terms with one query each.
func (s *Server) toItems(
	db *gorm.DB,
	groups []*group,
	edges map[pb.ActivityKind]time.Time,
	requestLocale string,
) ([]*pb.ActivityItem, error) {
	userIDs, localeIDs, entryIDs, termIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, g := range groups {
		if g.Newest.UserID != "" {
			userIDs[g.Newest.UserID] = true
		}
		for localeID := range g.Locales {
			localeIDs[localeID] = true
		}
		if len(g.Subjects) == 1 {
			if g.Newest.Kind == pb.ActivityKind_ACTIVITY_KIND_TRANSLATION {
				entryIDs[g.Newest.SubjectID] = true
			} else {
				termIDs[g.Newest.SubjectID] = true
			}
		}
	}

	var users []models.User
	var locales []models.Locale
	var entries []models.Entry
	var terms []models.Terminology
	// Deleted records keep their names so older activity stays readable.
	for _, lookup := range []struct {
		ids  map[string]bool
		dest any
	}{{userIDs, &users}, {localeIDs, &locales}, {entryIDs, &entries}, {termIDs, &terms}} {
		if len(lookup.ids) == 0 {
			continue
		}
		if err := db.Unscoped().Where("id IN ?", keys(lookup.ids)).Find(lookup.dest).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load activity details: %w", err))
		}
	}

	userNames := map[string]string{}
	for _, u := range users {
		userNames[u.ID] = u.Name
	}
	localeByID := map[string]*pb.Locale{}
	for i := range locales {
		localeByID[locales[i].ID] = converter.LocaleToProto(&locales[i], requestLocale)
	}
	labels := map[string]string{}
	for _, e := range entries {
		labels[e.ID] = e.Key
	}
	for _, term := range terms {
		labels[term.ID] = term.Term
	}

	items := make([]*pb.ActivityItem, 0, len(groups))
	for _, g := range groups {
		item := &pb.ActivityItem{
			Id:           g.Newest.ID,
			Kind:         g.Newest.Kind,
			CreatedCount: int32(g.Created),
			UpdatedCount: int32(g.Updated),
			DeletedCount: int32(g.Deleted),
			SubjectCount: int32(len(g.Subjects)),
			LocaleCount:  int32(len(g.Locales)),
			UserId:       g.Newest.UserID,
			UserName:     userNames[g.Newest.UserID],
			StartedAt:    g.Oldest.Format(time.RFC3339),
			ChangedAt:    g.Newest.ChangedAt.Format(time.RFC3339),
		}
		if edge, ok := edges[g.Newest.Kind]; ok && !g.Oldest.After(edge) {
			item.Partial = true
		}
		if len(g.Locales) == 1 {
			item.LocaleId = g.Newest.LocaleID
			if locale := localeByID[g.Newest.LocaleID]; locale != nil {
				item.LocaleCode = locale.Code
				item.LocaleName = locale.Name
			}
		}
		if len(g.Subjects) == 1 {
			item.SubjectId = g.Newest.SubjectID
			item.SubjectLabel = labels[g.Newest.SubjectID]
		}
		items = append(items, item)
	}
	return items, nil
}

func keys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	return out
}
