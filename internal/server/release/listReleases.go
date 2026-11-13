package release

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListReleases lists pending releases by date, unscheduled ones last, then released
// ones, the latest first
func (s *Server) ListReleases(
	ctx context.Context,
	req *connect.Request[pb.ListReleasesRequest],
) (*connect.Response[pb.ListReleasesResponse], error) {
	db := s.db.WithContext(ctx)
	query := db.Model(&models.Release{})

	switch req.Msg.State {
	case pb.ReleaseState_RELEASE_STATE_UNSPECIFIED:
	case pb.ReleaseState_RELEASE_STATE_PENDING:
		query = query.Where("released_at IS NULL")
	case pb.ReleaseState_RELEASE_STATE_RELEASED:
		query = query.Where("released_at IS NOT NULL")
	default:
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "unknown release state")
	}

	// The CASEs order pending releases by date and leave released ones to released_at.
	query = query.
		Order("released_at IS NOT NULL").
		Order("CASE WHEN released_at IS NULL THEN release_at IS NULL END").
		Order("CASE WHEN released_at IS NULL THEN release_at END").
		Order("released_at DESC").
		Order("LOWER(name)")

	var releases []models.Release

	pagination, err := common.Paginate(query, req.Msg.Pagination, &releases)
	if err != nil {
		return nil, err
	}

	result, err := releasesToProto(db, releases)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.ListReleasesResponse{
		Releases:   result,
		Pagination: pagination,
	}), nil
}
