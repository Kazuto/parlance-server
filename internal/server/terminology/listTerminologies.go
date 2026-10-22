package terminology

import (
	"context"
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

func (s *Server) ListTerminologies(
	ctx context.Context,
	req *connect.Request[pb.ListTerminologiesRequest],
) (*connect.Response[pb.ListTerminologiesResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadTerminology); err != nil {
		return nil, err
	}

	page := int32(1)
	perPage := int32(50)

	if req.Msg.Pagination != nil {
		if req.Msg.Pagination.Page > 0 {
			page = req.Msg.Pagination.Page
		}
		if req.Msg.Pagination.PerPage > 0 {
			perPage = req.Msg.Pagination.PerPage
		}
	}

	var terminologies []models.Terminology
	var total int64

	query := s.db.Model(&models.Terminology{})

	allowedSorts := map[string]bool{"term": true, "created_at": true, "updated_at": true}
	query, err := common.NewFilter(query, req.Msg.Filter).
		AllowedSorts(allowedSorts).
		DefaultSort("term ASC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return searchTerminologies(query, search, req.Msg.TargetLocaleIds)
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, err.Error())
	}

	offset := (page - 1) * perPage
	if err := query.Preload("Definitions").
		Limit(int(perPage)).
		Offset(int(offset)).
		Find(&terminologies).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, err.Error())
	}

	return connect.NewResponse(&pb.ListTerminologiesResponse{
		Terminologies: converter.TerminologiesToProto(terminologies),
		Pagination:    converter.PaginationToProto(total, page, perPage),
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
