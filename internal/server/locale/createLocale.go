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

// CreateLocale creates a new locale
func (s *Server) CreateLocale(
	ctx context.Context,
	req *connect.Request[pb.CreateLocaleRequest],
) (*connect.Response[pb.CreateLocaleResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.CreateLocale); err != nil {
		return nil, err
	}

	if req.Msg.Code == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale code is required")
	}

	if len(req.Msg.GetNames().GetValues()) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale names are required")
	}

	names, err := converter.NamesMapToJSON(req.Msg.GetNames().GetValues())
	if err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to encode names")
	}

	// If setting as default, unset other defaults
	if req.Msg.IsDefault {
		if err := s.db.Model(&models.Locale{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return nil, errors.Internal(errors.ErrInternal, "failed to update default locale")
		}
	}

	locale := &models.Locale{
		Code:      req.Msg.Code,
		Names:     names,
		IsDefault: req.Msg.IsDefault,
	}

	if err := s.db.Create(locale).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create locale")
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.CreateLocaleResponse{
		Locale: converter.LocaleToProto(locale, requestLocale),
	}), nil
}
