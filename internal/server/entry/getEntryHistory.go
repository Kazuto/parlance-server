package entry

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetEntryHistory returns the localization history for an entry
func (s *Server) GetEntryHistory(
	ctx context.Context,
	req *connect.Request[pb.GetEntryHistoryRequest],
) (*connect.Response[pb.GetEntryHistoryResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadEntry); err != nil {
		return nil, err
	}

	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var history []models.LocalizationHistory
	var total int64

	query := s.db.Model(&models.LocalizationHistory{}).
		Preload("User").
		Where("entry_id = ?", req.Msg.EntryId)

	if err := query.Count(&total).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to count history")
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Order("changed_at DESC").Find(&history).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to fetch history")
	}

	return connect.NewResponse(&pb.GetEntryHistoryResponse{
		History:    converter.LocalizationHistoriesToProto(history),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}
