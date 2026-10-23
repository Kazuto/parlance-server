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

// GetLocale returns a single locale by ID
func (s *Server) GetLocale(
	ctx context.Context,
	req *connect.Request[pb.GetLocaleRequest],
) (*connect.Response[pb.GetLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	locale, err := common.FindByID[models.Locale](s.db.WithContext(ctx), req.Msg.Id, errors.ErrLocaleNotFound, "locale not found")
	if err != nil {
		return nil, err
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.GetLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
