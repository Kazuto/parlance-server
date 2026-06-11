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

// CreateLocale creates a new locale
func (s *Server) CreateLocale(
	ctx context.Context,
	req *connect.Request[pb.CreateLocaleRequest],
) (*connect.Response[pb.CreateLocaleResponse], error) {
	if req.Msg.Code == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale code is required"))
	}

	if len(req.Msg.Names) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale names are required"))
	}

	names, err := converter.NamesMapToJSON(req.Msg.Names)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to encode names: %w", err))
	}

	// If setting as default, unset other defaults
	if req.Msg.IsDefault {
		if err := s.db.Model(&models.Locale{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update default locale: %w", err))
		}
	}

	locale := &models.Locale{
		Code:      req.Msg.Code,
		Names:     names,
		IsDefault: req.Msg.IsDefault,
	}

	if err := s.db.Create(locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create locale: %w", err))
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.CreateLocaleResponse{
		Locale: converter.LocaleToProto(locale, requestLocale),
	}), nil
}
