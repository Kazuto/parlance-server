package entry

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetEntryHistory returns the localization history for an entry
func (s *Server) GetEntryHistory(
	ctx context.Context,
	req *connect.Request[pb.GetEntryHistoryRequest],
) (*connect.Response[pb.GetEntryHistoryResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	query := s.db.WithContext(ctx).Model(&models.LocalizationHistory{}).
		Where("entry_id = ?", req.Msg.EntryId).
		Order("changed_at DESC")
	if req.Msg.LocaleId != "" {
		query = query.Where("locale_id = ?", req.Msg.LocaleId)
	}

	if req.Msg.Before != "" {
		before, err := time.Parse(time.RFC3339, req.Msg.Before)
		if err != nil {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "before must be an RFC 3339 time")
		}

		query = query.Where("changed_at < ?", before)
	}

	var history []models.LocalizationHistory

	pagination, err := common.Paginate(query, req.Msg.Pagination, &history, "User")
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetEntryHistoryResponse{
		History:    converter.LocalizationHistoriesToProto(history),
		Pagination: pagination,
	}), nil
}
