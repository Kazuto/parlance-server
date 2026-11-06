package localization

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListLocalizations returns a paginated list of localizations for an entry
func (s *Server) ListLocalizations(
	ctx context.Context,
	req *connect.Request[pb.ListLocalizationsRequest],
) (*connect.Response[pb.ListLocalizationsResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	query := s.db.WithContext(ctx).Model(&models.Localization{}).Where("entry_id = ?", req.Msg.EntryId)

	query, err := common.NewFilter(query, req.Msg.Filter).
		IncludeDeletedRequires(ctx, s.db, permissions.UpdateEntry).
		AllowedSorts(map[string]bool{"created_at": true, "created_by": true, "updated_at": true, "updated_by": true}).
		DefaultSort("created_at DESC").
		Search(common.SearchColumns("translation")).
		Apply()
	if err != nil {
		return nil, err
	}

	var localizations []models.Localization

	pagination, err := common.Paginate(query, req.Msg.Pagination, &localizations)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ListLocalizationsResponse{
		Localizations: converter.LocalizationsToProto(localizations),
		Pagination:    pagination,
	}), nil
}
