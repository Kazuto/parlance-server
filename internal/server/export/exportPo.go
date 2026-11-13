package export

import (
	"context"

	"connectrpc.com/connect"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ExportPo(ctx context.Context, req *connect.Request[pb.ExportPoRequest]) (*connect.Response[pb.ExportPoResponse], error) {
	content, filename, err := s.exportFile(ctx, FormatPO, req.Msg.LocaleCode, req.Msg)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ExportPoResponse{Format: FormatPO, Content: content, Filename: filename}), nil
}
