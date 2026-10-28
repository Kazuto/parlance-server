package translationserver

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/settings"
	"github.com/kazuto/parlance-server/internal/translate"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// maxStyleLength limits the style, which goes into every prompt.
const maxStyleLength = 200

// GetTranslationSettings returns the style machine translations are asked for
func (s *Server) GetTranslationSettings(
	ctx context.Context,
	req *connect.Request[pb.GetTranslationSettingsRequest],
) (*connect.Response[pb.GetTranslationSettingsResponse], error) {
	result, err := s.translationSettings(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetTranslationSettingsResponse{Settings: result}), nil
}

// UpdateTranslationSettings changes the style machine translations are asked for. The
// prompt puts it in square brackets, so it is one line without any.
func (s *Server) UpdateTranslationSettings(
	ctx context.Context,
	req *connect.Request[pb.UpdateTranslationSettingsRequest],
) (*connect.Response[pb.UpdateTranslationSettingsResponse], error) {
	style := strings.TrimSpace(req.Msg.Style)

	if utf8.RuneCountInString(style) > maxStyleLength || strings.ContainsAny(style, "[]\r\n") {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "the style must be one line of at most 200 characters, without square brackets")
	}

	if err := settings.SetString(s.db.WithContext(ctx), models.SettingTranslationStyle, style, middleware.UserIDPtr(ctx)); err != nil {
		return nil, errors.InternalCause(err, "failed to save the translation settings")
	}

	result, err := s.translationSettings(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.UpdateTranslationSettingsResponse{Settings: result}), nil
}

func (s *Server) translationSettings(ctx context.Context) (*pb.TranslationSettings, error) {
	style, updatedAt, err := settings.String(s.db.WithContext(ctx), models.SettingTranslationStyle, "")
	if err != nil {
		return nil, errors.InternalCause(err, "failed to load the translation settings")
	}

	result := &pb.TranslationSettings{Style: style, DefaultStyle: translate.DefaultStyle}

	if !updatedAt.IsZero() {
		formatted := updatedAt.UTC().Format(time.RFC3339)
		result.UpdatedAt = &formatted
	}

	return result, nil
}
