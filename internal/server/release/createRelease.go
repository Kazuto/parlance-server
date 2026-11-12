package release

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateRelease creates a pending release, with or without a date
func (s *Server) CreateRelease(
	ctx context.Context,
	req *connect.Request[pb.CreateReleaseRequest],
) (*connect.Response[pb.CreateReleaseResponse], error) {
	name, err := releaseName(req.Msg.Name)
	if err != nil {
		return nil, err
	}

	userID := middleware.UserIDPtr(ctx)
	release := &models.Release{Name: name, Description: req.Msg.Description, CreatedBy: userID, UpdatedBy: userID}

	if req.Msg.ReleaseAt != nil {
		if release.ReleaseAt, err = releaseAt(req.Msg.GetReleaseAt()); err != nil {
			return nil, err
		}
	}

	db := s.db.WithContext(ctx)

	if err := checkReleaseName(db, name, ""); err != nil {
		return nil, err
	}

	if err := db.Create(release).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to create release")
	}

	return connect.NewResponse(&pb.CreateReleaseResponse{
		Release: converter.ReleaseToProto(release),
	}), nil
}
