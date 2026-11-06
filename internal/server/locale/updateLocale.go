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

	if req.Msg.Names != nil && len(req.Msg.Names.GetValues()) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale names are required")
	}

	formality, ok := converter.FormalityFromProto(req.Msg.GetFormality())
	if !ok {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "unknown formality")
	}

	locale, err := common.FindByID[models.Locale](s.db.WithContext(ctx), req.Msg.Id, errors.ErrLocaleNotFound, "locale not found")
	if err != nil {
		return nil, err
	}

	if req.Msg.Code != nil {
		locale.Code = req.Msg.GetCode()
	}

	if req.Msg.Names != nil {
		names, err := converter.NamesMapToJSON(req.Msg.Names.GetValues())
		if err != nil {
			return nil, errors.Internal(errors.ErrInternal, "failed to encode names")
		}

		locale.Names = names
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
