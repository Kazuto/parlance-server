package locale

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetDefaultLocale returns the default locale
func (s *Server) GetDefaultLocale(
	ctx context.Context,
	req *connect.Request[pb.GetDefaultLocaleRequest],
) (*connect.Response[pb.GetDefaultLocaleResponse], error) {
	var locale models.Locale
	if err := s.db.Where("is_default = ?", true).First(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("default locale not found"))
	}

	requestLocale := getLocaleFromRequest(req)

	return connect.NewResponse(&pb.GetDefaultLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
