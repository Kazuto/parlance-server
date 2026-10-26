package translationserver

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/settings"
	"github.com/kazuto/parlance-server/internal/translate"
	"github.com/kazuto/parlance-server/internal/translators"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// testSample is what TestTranslationServer translates, in the style admins set.
var testSample = translate.Request{Text: "Save changes", TargetLang: "German"}

// TestTranslationServer translates a short sample with one server, whether it is
// enabled or not, and reports the answer or what went wrong.
func (s *Server) TestTranslationServer(
	ctx context.Context,
	req *connect.Request[pb.TestTranslationServerRequest],
) (*connect.Response[pb.TestTranslationServerResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "translation server id is required")
	}

	server, err := common.FindByID[models.TranslationServer](s.db.WithContext(ctx), req.Msg.Id, errors.ErrTranslationServerNotFound, "translation server not found")
	if err != nil {
		return nil, err
	}

	result := &pb.TestTranslationServerResponse{}
	endpoint, err := translators.Endpoint(server, s.box)

	if err != nil {
		result.Error = err.Error()

		return connect.NewResponse(result), nil
	}

	style, _, err := settings.String(s.db.WithContext(ctx), models.SettingTranslationStyle, "")
	if err != nil {
		return nil, errors.InternalCause(err, "failed to load the translation style")
	}

	sample := testSample
	sample.Style = style

	started := time.Now()
	answer, err := s.newTranslator(endpoint).Translate(ctx, sample)
	result.DurationMs = int32(time.Since(started).Milliseconds())

	var translation string
	if err == nil {
		translation, _, err = translate.Clean(sample.Text, answer.Text)
	}

	if err != nil {
		result.Error = err.Error()

		return connect.NewResponse(result), nil
	}

	result.Ok = true
	result.Translation = translation

	return connect.NewResponse(result), nil
}
