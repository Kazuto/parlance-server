package importservice

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
)

func (s *Server) ImportJson(
	ctx context.Context,
	req *connect.Request[pb.ImportJsonRequest],
) (*connect.Response[pb.ImportJsonResponse], error) {
	options, err := s.startImport(ctx, req.Msg.Mode, req.Msg.Approve, req.Msg.DryRun, req.Msg.FileContent)
	if err != nil {
		return nil, err
	}

	if err := requireLocaleID(req.Msg.LocaleId); err != nil {
		return nil, err
	}

	// Parse JSON content from uploaded file
	translations, err := parseJSONTranslations(req.Msg.FileContent)
	if err != nil {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "invalid JSON format")
	}

	// Process import
	result, err := s.processImport(ctx, req.Msg.LocaleId, req.Msg.ScopeIds, translations, options)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ImportJsonResponse{
		Result: result,
	}), nil
}
