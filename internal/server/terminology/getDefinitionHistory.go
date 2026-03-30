package terminology

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) GetDefinitionHistory(
	ctx context.Context,
	req *connect.Request[pb.GetDefinitionHistoryRequest],
) (*connect.Response[pb.GetDefinitionHistoryResponse], error) {
	if req.Msg.TerminologyId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("terminology id is required"))
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

	var history []models.DefinitionHistory
	var total int64

	offset := (page - 1) * perPage

	query := s.db.Model(&models.DefinitionHistory{}).
		Where("terminology_id = ?", req.Msg.TerminologyId)

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := query.Preload("User").
		Order("changed_at DESC").
		Limit(int(perPage)).
		Offset(int(offset)).
		Find(&history).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.GetDefinitionHistoryResponse{
		History:    converter.DefinitionHistoriesToProto(history),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
