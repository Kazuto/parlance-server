package locale

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
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
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	locale, err := common.FindByID[models.Locale](s.db.WithContext(ctx), req.Msg.LocaleId, errors.ErrLocaleNotFound, "locale not found")
	if err != nil {
		return nil, err
	}

	locale.IsDefault = true
	if err := s.saveLocale(ctx, &locale); err != nil {
		return nil, err
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.SetDefaultLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
