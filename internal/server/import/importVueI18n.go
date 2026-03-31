package importservice

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ImportVueI18N(
	ctx context.Context,
	req *connect.Request[pb.ImportVueI18NRequest],
) (*connect.Response[pb.ImportVueI18NResponse], error) {
	if req.Msg.LocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale_id is required"))
	}

	if len(req.Msg.FileContent) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("file_content is required"))
	}

	// Parse nested JSON content
	var nestedData map[string]interface{}
	if err := json.Unmarshal(req.Msg.FileContent, &nestedData); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid JSON format: %w", err))
	}

	// Flatten nested structure to key-value pairs
	translations := make(map[string]string)
	flattenJSON(nestedData, "", translations)

	// Set default mode if not specified
	mode := req.Msg.Mode
	if mode == pb.ImportMode_IMPORT_MODE_UNSPECIFIED {
		mode = pb.ImportMode_CREATE_OR_UPDATE
	}

	// Process import
	result, err := s.processImport(ctx, req.Msg.LocaleId, req.Msg.ScopeIds, translations, mode)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&pb.ImportVueI18NResponse{
		Result: result,
	}), nil
}

// flattenJSON converts nested JSON to flat key-value pairs with dot notation
// Example: {"user": {"name": "John"}} -> {"user.name": "John"}
func flattenJSON(data map[string]interface{}, prefix string, result map[string]string) {
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case string:
			result[fullKey] = v
		case map[string]interface{}:
			flattenJSON(v, fullKey, result)
		case float64, int, bool:
			result[fullKey] = fmt.Sprintf("%v", v)
		default:
			// Skip arrays and other complex types
			continue
		}
	}
}
