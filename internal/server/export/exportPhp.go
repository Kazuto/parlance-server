package export

import (
	"context"

	"connectrpc.com/connect"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func (s *Server) ExportPhp(
	ctx context.Context,
	req *connect.Request[pb.ExportPhpRequest],
) (*connect.Response[pb.ExportPhpResponse], error) {
	content, filename, err := s.exportFile(ctx, FormatPHP, req.Msg.LocaleCode, req.Msg)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ExportPhpResponse{
		Format:   FormatPHP,
		Content:  content,
		Filename: filename,
	}), nil
}
