package locale

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// SetDefaultLocale sets a locale as the default
func (s *Server) SetDefaultLocale(
	ctx context.Context,
	req *connect.Request[pb.SetDefaultLocaleRequest],
) (*connect.Response[pb.SetDefaultLocaleResponse], error) {
	if req.Msg.LocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.LocaleId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	// Unset other defaults
	if err := s.db.Model(&models.Locale{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update default locale: %w", err))
	}

	// Set this as default
	locale.IsDefault = true
	if err := s.db.Save(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to set default locale: %w", err))
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.SetDefaultLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
