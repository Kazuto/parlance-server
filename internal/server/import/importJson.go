package importservice

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ImportJson(
	ctx context.Context,
	req *connect.Request[pb.ImportJsonRequest],
) (*connect.Response[pb.ImportJsonResponse], error) {
	if req.Msg.LocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale_id is required"))
	}

	if len(req.Msg.FileContent) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("file_content is required"))
	}

	// Parse JSON content from uploaded file
	var translations map[string]string
	if err := json.Unmarshal(req.Msg.FileContent, &translations); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid JSON format: %w", err))
	}

	// Set default mode if not specified
	mode := req.Msg.Mode
	if mode == pb.ImportMode_IMPORT_MODE_UNSPECIFIED {
		mode = pb.ImportMode_IMPORT_MODE_CREATE_OR_UPDATE
	}

	// Process import
	result, err := s.processImport(ctx, req.Msg.LocaleId, req.Msg.ScopeIds, translations, mode)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.ImportJsonResponse{
		Result: result,
	}), nil
}
