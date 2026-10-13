package importservice

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/gettext"
)

func (s *Server) ImportPo(ctx context.Context, req *connect.Request[pb.ImportPoRequest]) (*connect.Response[pb.ImportPoResponse], error) {
	if req.Msg.LocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale_id is required"))
	}
	if len(req.Msg.FileContent) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("file_content is required"))
	}
	entries, err := gettext.Parse(req.Msg.FileContent)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid PO format: %w", err))
	}
	translations := map[string]string{}
	seen := map[string]bool{}
	var failures []*pb.ImportError
	var skipped int32
	for _, entry := range entries {
		if entry.Obsolete {
			skipped++
			continue
		}
		var reason string
		switch {
		case entry.Context:
			reason = "msgctxt is not supported by key-based PO imports"
		case entry.Plural:
			reason = "plural forms are not supported"
		case entry.Fuzzy:
			reason = "fuzzy translations require review and are not imported"
		case seen[entry.ID]:
			reason = "duplicate msgid"
		}
		if reason != "" {
			failures = append(failures, &pb.ImportError{Key: entry.ID, Message: reason})
			continue
		}
		seen[entry.ID] = true
		if entry.Translation == "" {
			skipped++ // Untranslated gettext records must not erase existing translations.
			continue
		}
		translations[entry.ID] = entry.Translation
	}
	mode := req.Msg.Mode
	if mode == pb.ImportMode_IMPORT_MODE_UNSPECIFIED {
		mode = pb.ImportMode_IMPORT_MODE_CREATE_OR_UPDATE
	}
	if mode == pb.ImportMode_IMPORT_MODE_REPLACE_ALL && len(failures) > 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("replace-all requires a PO file without unsupported or duplicate entries"))
	}
	result, err := s.processImport(ctx, req.Msg.LocaleId, req.Msg.ScopeIds, translations, mode)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	result.EntriesSkipped += skipped
	result.EntriesFailed += int32(len(failures))
	result.Errors = append(result.Errors, failures...)
	return connect.NewResponse(&pb.ImportPoResponse{Result: result}), nil
}
