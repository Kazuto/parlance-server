package importservice

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ImportPhp(
	ctx context.Context,
	req *connect.Request[pb.ImportPhpRequest],
) (*connect.Response[pb.ImportPhpResponse], error) {
	if req.Msg.LocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale_id is required"))
	}

	if len(req.Msg.FileContent) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("file_content is required"))
	}

	// Parse PHP array content
	content := string(req.Msg.FileContent)
	translations, err := parsePHPArray(content)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid PHP array format: %w", err))
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

	return connect.NewResponse(&pb.ImportPhpResponse{
		Result: result,
	}), nil
}

// parsePHPArray parses Laravel PHP array format into key-value pairs
// Example: ['key' => 'value', "key2" => "value2"]
func parsePHPArray(content string) (map[string]string, error) {
	translations := make(map[string]string)

	// Remove PHP opening tag, return statement, and semicolon
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "<?php")
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "return")
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "[")
	content = strings.TrimSuffix(content, "]")
	content = strings.TrimSuffix(content, ";")
	content = strings.TrimSpace(content)

	// Match key-value pairs: 'key' => 'value' or "key" => "value"
	// This regex handles both single and double quotes, and escaped quotes
	re := regexp.MustCompile(`['"]([^'"\\]*(\\.[^'"\\]*)*)['"][\s]*=>[\s]*['"]([^'"\\]*(\\.[^'"\\]*)*)['"]`)
	matches := re.FindAllStringSubmatch(content, -1)

	if len(matches) == 0 {
		return nil, fmt.Errorf("no valid key-value pairs found in PHP array")
	}

	for _, match := range matches {
		if len(match) >= 4 {
			key := match[1]
			value := match[3]

			// Unescape quotes
			key = strings.ReplaceAll(key, `\'`, `'`)
			key = strings.ReplaceAll(key, `\"`, `"`)
			value = strings.ReplaceAll(value, `\'`, `'`)
			value = strings.ReplaceAll(value, `\"`, `"`)

			translations[key] = value
		}
	}

	return translations, nil
}
