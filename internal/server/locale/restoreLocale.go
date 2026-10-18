package locale

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

func (s *Server) RestoreLocale(
	ctx context.Context,
	req *connect.Request[pb.RestoreLocaleRequest],
) (*connect.Response[pb.RestoreLocaleResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteLocale); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	var locale models.Locale
	if err := s.db.Unscoped().First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocaleNotFound, "locale not found")
	}

	locale.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&locale).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to restore locale")
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.RestoreLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
