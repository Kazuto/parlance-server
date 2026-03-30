package terminology

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

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

	offset := (page - 1) * perPage

	if err := s.db.Model(&models.Terminology{}).Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := s.db.Preload("Definitions").
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
