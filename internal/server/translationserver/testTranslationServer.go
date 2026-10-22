package translationserver

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/translate"
	"github.com/kazuto/parlance-server/internal/translators"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// testSample is what TestTranslationServer translates.
var testSample = translate.Request{Text: "Save changes", TargetLang: "German"}

// TestTranslationServer translates a short sample with one server, whether it is
// enabled or not, and reports the answer or what went wrong.
func (s *Server) TestTranslationServer(
	ctx context.Context,
	req *connect.Request[pb.TestTranslationServerRequest],
) (*connect.Response[pb.TestTranslationServerResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateTranslationServer); err != nil {
		return nil, err
	}

	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "translation server id is required")
	}

	var server models.TranslationServer
	if err := s.db.First(&server, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, errors.NotFound(errors.ErrTranslationServerNotFound, "translation server not found")
	}

	result := &pb.TestTranslationServerResponse{}
	endpoint, err := translators.Endpoint(server, s.box)
	if err != nil {
		result.Error = err.Error()
		return connect.NewResponse(result), nil
	}

	started := time.Now()
	answer, err := s.newTranslator(endpoint).Translate(ctx, testSample)
	result.DurationMs = int32(time.Since(started).Milliseconds())
	if err == nil {
		answer, _, err = translate.Clean(testSample.Text, answer)
	}
	if err != nil {
		result.Error = err.Error()
		return connect.NewResponse(result), nil
	}

	result.Ok = true
	result.Translation = answer
	return connect.NewResponse(result), nil
}
