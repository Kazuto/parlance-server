package release

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteRelease deletes a release for good. A pending release's entries export again
// unless another release holds them.
func (s *Server) DeleteRelease(
	ctx context.Context,
	req *connect.Request[pb.DeleteReleaseRequest],
) (*connect.Response[pb.DeleteReleaseResponse], error) {
	db := s.db.WithContext(ctx)

	release, err := findRelease(db, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	// The foreign key cascades too; deleting the links first keeps it explicit.
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("release_id = ?", release.ID).Delete(&models.ReleaseEntry{}).Error; err != nil {
			return err
		}

		return tx.Delete(&release).Error
	}); err != nil {
		return nil, errors.InternalCause(err, "failed to delete release")
	}

	return connect.NewResponse(&pb.DeleteReleaseResponse{}), nil
}
