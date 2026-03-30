package locale

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetLocale returns a single locale by ID
func (s *Server) GetLocale(
	ctx context.Context,
	req *connect.Request[pb.GetLocaleRequest],
) (*connect.Response[pb.GetLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	requestLocale := getLocaleFromRequest(req)

	return connect.NewResponse(&pb.GetLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
