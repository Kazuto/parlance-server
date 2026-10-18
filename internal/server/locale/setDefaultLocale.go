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

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// SetDefaultLocale sets a locale as the default
func (s *Server) SetDefaultLocale(
	ctx context.Context,
	req *connect.Request[pb.SetDefaultLocaleRequest],
) (*connect.Response[pb.SetDefaultLocaleResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateLocale); err != nil {
		return nil, err
	}

	if req.Msg.LocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.LocaleId).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocaleNotFound, "locale not found")
	}

	// Unset other defaults
	if err := s.db.Model(&models.Locale{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update default locale")
	}

	// Set this as default
	locale.IsDefault = true
	if err := s.db.Save(&locale).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to set default locale")
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.SetDefaultLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
