package localization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListLocalizations returns a paginated list of localizations for an entry
func (s *Server) ListLocalizations(
	ctx context.Context,
	req *connect.Request[pb.ListLocalizationsRequest],
) (*connect.Response[pb.ListLocalizationsResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var localizations []models.Localization
	var total int64

	query := s.db.Model(&models.Localization{}).Where("entry_id = ?", req.Msg.EntryId)

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count localizations: %w", err))
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Order("created_at DESC").Find(&localizations).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch localizations: %w", err))
	}

	return connect.NewResponse(&pb.ListLocalizationsResponse{
		Localizations: converter.LocalizationsToProto(localizations),
		Pagination:    converter.PaginationToProto(total, page, perPage),
	}), nil
}
