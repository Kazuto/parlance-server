package terminology

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) GetDefinitionHistory(
	ctx context.Context,
	req *connect.Request[pb.GetDefinitionHistoryRequest],
) (*connect.Response[pb.GetDefinitionHistoryResponse], error) {
	if req.Msg.TerminologyId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "terminology id is required")
	}

	query := s.db.WithContext(ctx).Model(&models.DefinitionHistory{}).
		Where("terminology_id = ?", req.Msg.TerminologyId).
		Order("changed_at DESC")
	if req.Msg.LocaleId != "" {
		query = query.Where("locale_id = ?", req.Msg.LocaleId)
	}

	var history []models.DefinitionHistory

	pagination, err := common.Paginate(query, req.Msg.Pagination, &history, "User")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetDefinitionHistoryResponse{
		History:    converter.DefinitionHistoriesToProto(history),
		Pagination: pagination,
	}), nil
}
