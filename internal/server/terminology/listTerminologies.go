package terminology

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ListTerminologies(
	ctx context.Context,
	req *connect.Request[pb.ListTerminologiesRequest],
) (*connect.Response[pb.ListTerminologiesResponse], error) {
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

	allowedSorts := map[string]bool{"name": true, "slug": true, "created_at": true, "updated_at": true}
	query, err := common.NewFilter(query, req.Msg.Filter).
		AllowedSorts(allowedSorts).
		DefaultSort("term ASC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return query.Where("name LIKE ?", "%"+search+"%").Where("slug LIKE ?", "%"+search+"%")
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	offset := (page - 1) * perPage
	if err := query.Preload("Definitions").
		Limit(int(perPage)).
		Offset(int(offset)).
		Find(&terminologies).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.ListTerminologiesResponse{
		Terminologies: converter.TerminologiesToProto(terminologies),
		Pagination:    converter.PaginationToProto(total, page, perPage),
	}), nil
}
