package update

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/updates"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// GetUpdates returns the newest releases of the server and the web app
func (s *Server) GetUpdates(
	ctx context.Context,
	_ *connect.Request[pb.GetUpdatesRequest],
) (*connect.Response[pb.GetUpdatesResponse], error) {
	if s.checker == nil {
		return connect.NewResponse(&pb.GetUpdatesResponse{}), nil
	}

	result := s.checker.Latest(ctx)
	res := &pb.GetUpdatesResponse{
		Enabled: true,
		Server:  releaseToProto(result.Server),
		Web:     releaseToProto(result.Web),
	}

	if !result.CheckedAt.IsZero() {
		checkedAt := result.CheckedAt.UTC().Format(time.RFC3339)
		res.CheckedAt = &checkedAt
	}

	if result.Err != nil {
		res.Error = result.Err.Error()
	}

	return connect.NewResponse(res), nil
}

// releaseToProto leaves a release not found yet out.
func releaseToProto(release updates.Release) *pb.AvailableRelease {
	if release.Version == "" {
		return nil
	}

	return &pb.AvailableRelease{
		Version:     release.Version,
		Url:         release.URL,
		PublishedAt: release.PublishedAt.UTC().Format(time.RFC3339),
	}
}
