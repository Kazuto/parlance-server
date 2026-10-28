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

func (s *Server) RestoreLocale(
	ctx context.Context,
	req *connect.Request[pb.RestoreLocaleRequest],
) (*connect.Response[pb.RestoreLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	locale, err := common.Restore[models.Locale](ctx, s.db.DB, req.Msg.Id, errors.ErrLocaleNotFound, "locale not found")
	if err != nil {
		return nil, err
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.RestoreLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
