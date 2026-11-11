package export

import (
	"context"

	"connectrpc.com/connect"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ExportJson(
	ctx context.Context,
	req *connect.Request[pb.ExportJsonRequest],
) (*connect.Response[pb.ExportJsonResponse], error) {
	content, filename, err := s.exportFile(ctx, FormatJSON, req.Msg.LocaleCode, req.Msg.ScopeIds, req.Msg.TagIds, req.Msg.IncludeDrafts)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ExportJsonResponse{
		Format:   FormatJSON,
		Content:  content,
		Filename: filename,
	}), nil
}
