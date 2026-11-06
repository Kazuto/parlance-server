package export

import (
	"context"

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

	content, err := Render(db, FormatPHP, locale, scopeIDs, req.Msg.IncludeDrafts)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ExportPhpResponse{
		Format:   FormatPHP,
		Content:  content,
		Filename: exportFilename(db, scopeIDs, locale.Code, FormatPHP),
	}), nil
}
