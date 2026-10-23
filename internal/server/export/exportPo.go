package export

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/gettext"
	"github.com/kazuto/parlance-server/internal/server/common"
)

func (s *Server) ExportPo(ctx context.Context, req *connect.Request[pb.ExportPoRequest]) (*connect.Response[pb.ExportPoResponse], error) {
	db := s.db.WithContext(ctx)

	locale, err := findLocale(db, req.Msg.LocaleCode)
	if err != nil {
		return nil, err
	}

	scopeIDs := common.UniqueIDs(req.Msg.ScopeIds)

	entries, err := loadEntries(db, locale.ID, scopeIDs)
	if err != nil {
		return nil, err
	}

	messages := make([]gettext.Entry, len(entries))
	for i, entry := range entries {
		messages[i] = gettext.Entry{ID: entry.Key, Description: entry.Description, Translation: entry.Translation}
	}

	return connect.NewResponse(&pb.ExportPoResponse{
		Format: "po", Content: gettext.Marshal(locale.Code, messages), Filename: exportFilename(db, scopeIDs, locale.Code, "po"),
	}), nil
}
