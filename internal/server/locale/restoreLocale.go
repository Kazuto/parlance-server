package locale

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) RestoreLocale(
	ctx context.Context,
	req *connect.Request[pb.RestoreLocaleRequest],
) (*connect.Response[pb.RestoreLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.Id).Unscoped().Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	locale.DeletedAt = gorm.DeletedAt{Valid: false}
	if err := s.db.Save(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to restore locale: %w", err))
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.RestoreLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
