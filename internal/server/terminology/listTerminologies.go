package terminology

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ListTerminologies(
	ctx context.Context,
	req *connect.Request[pb.ListTerminologiesRequest],
) (*connect.Response[pb.ListTerminologiesResponse], error) {
	query, err := common.NewFilter(s.db.WithContext(ctx).Model(&models.Terminology{}), req.Msg.Filter).
		IncludeDeletedRequires(ctx, s.db, permissions.DeleteTerminology).
		AllowedSorts(map[string]bool{"term": true, "created_at": true, "updated_at": true}).
		DefaultSort("term ASC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return searchTerminologies(query, search, req.Msg.TargetLocaleIds)
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	var terminologies []models.Terminology

	pagination, err := common.Paginate(query, req.Msg.Pagination, &terminologies, "Definitions")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ListTerminologiesResponse{
		Terminologies: converter.TerminologiesToProto(terminologies),
		Pagination:    pagination,
	}), nil
}

// searchTerminologies keeps terms whose text or description, or whose definition in
// one of the given locales (any locale when none are given), contains search,
// ignoring case. The locales are the ones the glossary shows, so a match is
// visible once opened.
func searchTerminologies(query *gorm.DB, search string, localeIDs []string) *gorm.DB {
	pattern := "%" + strings.ToLower(search) + "%"
	// Start a separate statement so the correlated subquery doesn't inherit filters.
	definitions := query.Session(&gorm.Session{NewDB: true}).Table("definitions").Select("1").
		Where("definitions.terminology_id = terminologies.id AND definitions.deleted_at IS NULL AND LOWER(definitions.translation) LIKE ?", pattern)
	if len(localeIDs) > 0 {
		definitions = definitions.Where("definitions.locale_id IN ?", localeIDs)
	}

	return query.Where(
		"(LOWER(terminologies.term) LIKE ? OR LOWER(terminologies.description) LIKE ? OR EXISTS (?))",
		pattern, pattern, definitions,
	)
}
