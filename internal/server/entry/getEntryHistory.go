package entry

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetEntryHistory returns the localization history for an entry
func (s *Server) GetEntryHistory(
	ctx context.Context,
	req *connect.Request[pb.GetEntryHistoryRequest],
) (*connect.Response[pb.GetEntryHistoryResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var history []models.LocalizationHistory
	var total int64

	query := s.db.Model(&models.LocalizationHistory{}).
		Where("entry_id = ?", req.Msg.EntryId)

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count history: %w", err))
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Order("changed_at DESC").Find(&history).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch history: %w", err))
	}

	return connect.NewResponse(&pb.GetEntryHistoryResponse{
		History:    converter.LocalizationHistoriesToProto(history),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
