package locale

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// UpdateLocale updates an existing locale
func (s *Server) UpdateLocale(
	ctx context.Context,
	req *connect.Request[pb.UpdateLocaleRequest],
) (*connect.Response[pb.UpdateLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	// If setting as default, unset other defaults
	if req.Msg.IsDefault && !locale.IsDefault {
		if err := s.db.Model(&models.Locale{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update default locale: %w", err))
		}
	}

	if req.Msg.Code != "" {
		locale.Code = req.Msg.Code
	}

	if len(req.Msg.Names) > 0 {
		names, err := converter.NamesMapToJSON(req.Msg.Names)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to encode names: %w", err))
		}

		locale.Names = names
	}

	locale.IsDefault = req.Msg.IsDefault

	if err := s.db.Save(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update locale: %w", err))
	}

	requestLocale := getLocaleFromRequest(req)

	return connect.NewResponse(&pb.UpdateLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
