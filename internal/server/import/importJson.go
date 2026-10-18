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
	if req.Msg.LocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "locale_id is required")
	}

	if len(req.Msg.FileContent) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "file_content is required")
	}

	// Parse JSON content from uploaded file
	translations, err := parseJSONTranslations(req.Msg.FileContent)
	if err != nil {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "invalid JSON format")
	}

	// Set default mode if not specified
	mode := req.Msg.Mode
	if mode == pb.ImportMode_IMPORT_MODE_UNSPECIFIED {
		mode = pb.ImportMode_IMPORT_MODE_CREATE_OR_UPDATE
	}

	// Process import
	result, err := s.processImport(ctx, req.Msg.LocaleId, req.Msg.ScopeIds, translations, mode)
	if err != nil {
		return nil, errors.Internal(errors.ErrInternal, err.Error())
	}

	return connect.NewResponse(&pb.ImportJsonResponse{
		Result: result,
	}), nil
}
