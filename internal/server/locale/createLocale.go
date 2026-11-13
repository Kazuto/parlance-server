package locale

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/langnames"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateLocale creates a new locale
func (s *Server) CreateLocale(
	ctx context.Context,
	req *connect.Request[pb.CreateLocaleRequest],
) (*connect.Response[pb.CreateLocaleResponse], error) {
	if req.Msg.Code == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale code is required")
	}

	if err := checkCode(req.Msg.Code); err != nil {
		return nil, err
	}

	formality, ok := converter.FormalityFromProto(req.Msg.Formality)
	if !ok {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "unknown formality")
	}

	// Codes stay unique across deleted locales too.
	exists, err := common.Exists(s.db.WithContext(ctx).Unscoped().Model(&models.Locale{}).Where("code = ?", req.Msg.Code))
	if err != nil {
		return nil, errors.InternalCause(err, "failed to check locale code")
	}

	if exists {
		return nil, errors.AlreadyExists(errors.ErrLocaleAlreadyExists, "a locale with this code already exists")
	}

	locale := &models.Locale{
		Code:      req.Msg.Code,
		IsDefault: req.Msg.IsDefault,
		Formality: formality,
	}

	if err := s.saveLocale(ctx, locale); err != nil {
		return nil, err
	}

	requestLocale := common.GetLocaleFromRequest(req)

	return connect.NewResponse(&pb.CreateLocaleResponse{
		Locale: converter.LocaleToProto(locale, requestLocale),
	}), nil
}

// checkCode accepts the codes of known languages, which the locale's name comes from
func checkCode(code string) error {
	if !langnames.Valid(code) {
		return errors.InvalidInput(errors.ErrLocaleCodeUnknown, "locale code must name a known language, e.g. de or pt-BR")
	}

	return nil
}
