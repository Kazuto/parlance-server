package locale

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetDefaultLocale returns the default locale
func (s *Server) GetDefaultLocale(
	ctx context.Context,
	req *connect.Request[pb.GetDefaultLocaleRequest],
) (*connect.Response[pb.GetDefaultLocaleResponse], error) {
	locale, err := common.DefaultLocale(s.db.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	if locale == nil {
		return nil, errors.NotFound(errors.ErrLocaleNotFound, "default locale not found")
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.GetDefaultLocaleResponse{
		Locale: converter.LocaleToProto(locale, requestLocale),
	}), nil
}
