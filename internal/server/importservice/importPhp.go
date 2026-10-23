package importservice

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
)

func (s *Server) ImportPhp(
	ctx context.Context,
	req *connect.Request[pb.ImportPhpRequest],
) (*connect.Response[pb.ImportPhpResponse], error) {
	options, err := s.startImport(ctx, req.Msg.Mode, req.Msg.Approve, req.Msg.FileContent)
	if err != nil {
		return nil, err
	}

	if err := requireLocaleID(req.Msg.LocaleId); err != nil {
		return nil, err
	}

	// Parse PHP array content
	content := string(req.Msg.FileContent)

	translations, err := parsePHPArray(content)
	if err != nil {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "invalid PHP array format")
	}

	// Process import
	result, err := s.processImport(ctx, req.Msg.LocaleId, req.Msg.ScopeIds, translations, options)
	if err != nil {
		return nil, err
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

	matches := phpPairPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no valid key-value pairs found in PHP array")
	}

	for _, match := range matches {
		translations[unquotePHP(match[1])] = unquotePHP(match[2])
	}

	return translations, nil
}

// phpPairPattern matches 'key' => 'value' pairs of single- or double-quoted string literals
var phpPairPattern = regexp.MustCompile(`('(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*")\s*=>\s*('(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*")`)

// unquotePHP decodes a PHP string literal. Single-quoted strings only escape \\ and \';
// double-quoted ones also know the common control sequences.
func unquotePHP(literal string) string {
	quote, body := literal[0], literal[1:len(literal)-1]

	var out strings.Builder

	for i := 0; i < len(body); i++ {
		if body[i] != '\\' || i+1 == len(body) {
			out.WriteByte(body[i])

			continue
		}

		next := body[i+1]

		switch {
		case next == '\\' || next == quote:
			out.WriteByte(next)
		case quote == '"' && next == 'n':
			out.WriteByte('\n')
		case quote == '"' && next == 't':
			out.WriteByte('\t')
		case quote == '"' && next == '$':
			out.WriteByte('$')
		default:
			out.WriteByte('\\')
			out.WriteByte(next)
		}

		i++
	}

	return out.String()
}
