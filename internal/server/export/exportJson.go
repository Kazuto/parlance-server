package export

import (
	"context"
	"encoding/json"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ExportJson(
	ctx context.Context,
	req *connect.Request[pb.ExportJsonRequest],
) (*connect.Response[pb.ExportJsonResponse], error) {
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

	content, err := json.MarshalIndent(translationMap(entries), "", "  ")
	if err != nil {
		return nil, errors.InternalCause(err, "failed to marshal JSON")
	}

	return connect.NewResponse(&pb.ExportJsonResponse{
		Format:   "json",
		Content:  string(content),
		Filename: exportFilename(db, scopeIDs, locale.Code, "json"),
	}), nil
}
