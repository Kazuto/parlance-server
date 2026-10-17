package locale

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateLocale updates an existing locale
func (s *Server) UpdateLocale(
	ctx context.Context,
	req *connect.Request[pb.UpdateLocaleRequest],
) (*connect.Response[pb.UpdateLocaleResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateLocale); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}
	if req.Msg.Code != nil && req.Msg.GetCode() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale code is required"))
	}
	if req.Msg.Names != nil && len(req.Msg.Names.GetValues()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale names are required"))
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	// If setting as default, unset other defaults
	if req.Msg.GetIsDefault() && !locale.IsDefault {
		if err := s.db.Model(&models.Locale{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update default locale: %w", err))
		}
	}

	if req.Msg.Code != nil {
		locale.Code = req.Msg.GetCode()
	}

	if req.Msg.Names != nil {
		names, err := converter.NamesMapToJSON(req.Msg.Names.GetValues())
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to encode names: %w", err))
		}

		locale.Names = names
	}

	if req.Msg.IsDefault != nil {
		locale.IsDefault = req.Msg.GetIsDefault()
	}

	if err := s.db.Save(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update locale: %w", err))
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.UpdateLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
