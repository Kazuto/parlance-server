package release

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// errAlreadyReleased rejects moving the date of a release that went out
func errAlreadyReleased() error {
	return errors.FailedPrecondition(errors.ErrReleaseAlreadyReleased, "the release went out already, so its date can't change")
}

// UpdateRelease renames a release, describes it, or, while it's pending, moves or
// removes its date
func (s *Server) UpdateRelease(
	ctx context.Context,
	req *connect.Request[pb.UpdateReleaseRequest],
) (*connect.Response[pb.UpdateReleaseResponse], error) {
	if req.Msg.ReleaseAt != nil && req.Msg.ClearReleaseAt {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "a release date cannot be set and cleared at once")
	}

	db := s.db.WithContext(ctx)

	release, err := findRelease(db, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	if req.Msg.Name != nil {
		name, err := releaseName(req.Msg.GetName())
		if err != nil {
			return nil, err
		}

		if err := checkReleaseName(db, name, release.ID); err != nil {
			return nil, err
		}

		release.Name = name
	}

	if req.Msg.Description != nil {
		release.Description = req.Msg.GetDescription()
	}

	moved := req.Msg.ReleaseAt != nil || req.Msg.ClearReleaseAt
	if moved {
		if !release.Pending() {
			return nil, errAlreadyReleased()
		}

		release.ReleaseAt = nil

		if req.Msg.ReleaseAt != nil {
			if release.ReleaseAt, err = releaseAt(req.Msg.GetReleaseAt()); err != nil {
				return nil, err
			}
		}
	}

	release.UpdatedBy = middleware.UserIDPtr(ctx)

	// Only the columns set here, and the date only while the release is still pending.
	update := db.Model(&release).Select("name", "description", "release_at", "updated_by", "updated_at")
	if moved {
		update = update.Where("released_at IS NULL")
	}

	result := update.Updates(&release)
	if result.Error != nil {
		return nil, errors.InternalCause(result.Error, "failed to update release")
	}

	if result.RowsAffected == 0 {
		return nil, errAlreadyReleased()
	}

	updated, err := releaseToProto(db, release)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.UpdateReleaseResponse{Release: updated}), nil
}
