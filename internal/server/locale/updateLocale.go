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

// UpdateLocale updates an existing locale
func (s *Server) UpdateLocale(
	ctx context.Context,
	req *connect.Request[pb.UpdateLocaleRequest],
) (*connect.Response[pb.UpdateLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale id is required")
	}

	if req.Msg.Code != nil && req.Msg.GetCode() == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale code is required")
	}

	formality, ok := converter.FormalityFromProto(req.Msg.GetFormality())
	if !ok {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "unknown formality")
	}

	locale, err := common.FindByID[models.Locale](s.db.WithContext(ctx), req.Msg.Id, errors.ErrLocaleNotFound, "locale not found")
	if err != nil {
		return nil, err
	}

	// A locale with a made-up code from before codes were checked keeps it
	if req.Msg.Code != nil && req.Msg.GetCode() != locale.Code {
		if err := checkCode(req.Msg.GetCode()); err != nil {
			return nil, err
		}

		locale.Code = req.Msg.GetCode()
	}

	if req.Msg.IsDefault != nil {
		locale.IsDefault = req.Msg.GetIsDefault()
	}

	if req.Msg.Formality != nil {
		locale.Formality = formality
	}

	if err := s.saveLocale(ctx, &locale); err != nil {
		return nil, err
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.UpdateLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
