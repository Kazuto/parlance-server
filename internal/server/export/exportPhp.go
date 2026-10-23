package export

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ExportPhp(
	ctx context.Context,
	req *connect.Request[pb.ExportPhpRequest],
) (*connect.Response[pb.ExportPhpResponse], error) {
	db := s.db.WithContext(ctx)

	locale, err := findLocale(db, req.Msg.LocaleCode)
	if err != nil {
		return nil, err
	}

	scopeIDs := common.UniqueIDs(req.Msg.ScopeIds)

	entries, err := loadEntries(db, locale.ID, scopeIDs, req.Msg.IncludeDrafts)
	if err != nil {
		return nil, err
	}

	// Build Laravel PHP array format
	var content strings.Builder

	content.WriteString("<?php\n\nreturn [\n")

	for _, entry := range entries {
		if entry.Translated {
			fmt.Fprintf(&content, "    %s => %s,\n", phpString(entry.Key), phpString(entry.Translation))
		}
	}

	content.WriteString("];\n")

	return connect.NewResponse(&pb.ExportPhpResponse{
		Format:   "php",
		Content:  content.String(),
		Filename: exportFilename(db, scopeIDs, locale.Code, "php"),
	}), nil
}
