package localization

import (
	"context"

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

// ListLocalizations returns a paginated list of localizations for an entry
func (s *Server) ListLocalizations(
	ctx context.Context,
	req *connect.Request[pb.ListLocalizationsRequest],
) (*connect.Response[pb.ListLocalizationsResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadEntry); err != nil {
		return nil, err
	}

	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var localizations []models.Localization
	var total int64

	query := s.db.Model(&models.Localization{}).Where("entry_id = ?", req.Msg.EntryId)

	allowedSorts := map[string]bool{"created_at": true, "created_by": true, "updated_at": true, "updated_by": true}
	query, err := common.NewFilter(query, req.Msg.Filter).
		AllowedSorts(allowedSorts).
		DefaultSort("created_at DESC").
		Search(func(query *gorm.DB, search string) *gorm.DB {
			return query.Where("translation LIKE ?", "%"+search+"%")
		}).
		Apply()
	if err != nil {
		return nil, err
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to count localizations")
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Find(&localizations).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to fetch localizations")
	}

	return connect.NewResponse(&pb.ListLocalizationsResponse{
		Localizations: converter.LocalizationsToProto(localizations),
		Pagination:    converter.PaginationToProto(total, page, perPage),
	}), nil
}
