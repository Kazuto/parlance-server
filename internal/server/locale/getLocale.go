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

// GetLocale returns a single locale by ID
func (s *Server) GetLocale(
	ctx context.Context,
	req *connect.Request[pb.GetLocaleRequest],
) (*connect.Response[pb.GetLocaleResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.ReadLocale); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrLocaleNotFound, "locale not found")
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.GetLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
