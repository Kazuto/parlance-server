package release

import (
	"context"

	"connectrpc.com/connect"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetRelease returns a release
func (s *Server) GetRelease(
	ctx context.Context,
	req *connect.Request[pb.GetReleaseRequest],
) (*connect.Response[pb.GetReleaseResponse], error) {
	db := s.db.WithContext(ctx)

	release, err := findRelease(db, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	result, err := releaseToProto(db, release)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.GetReleaseResponse{Release: result}), nil
}
