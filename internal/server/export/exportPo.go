package export

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ExportPo(ctx context.Context, req *connect.Request[pb.ExportPoRequest]) (*connect.Response[pb.ExportPoResponse], error) {
	db := s.db.WithContext(ctx)

	locale, err := findLocale(db, req.Msg.LocaleCode)
	if err != nil {
		return nil, err
	}

	scopeIDs := common.UniqueIDs(req.Msg.ScopeIds)

	content, err := Render(db, FormatPO, locale, scopeIDs, req.Msg.IncludeDrafts)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ExportPoResponse{
		Format: FormatPO, Content: content, Filename: exportFilename(db, scopeIDs, locale.Code, FormatPO),
	}), nil
}
