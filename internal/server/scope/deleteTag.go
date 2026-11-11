package scope

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// DeleteTag deletes a tag for good and removes it from its entries
func (s *Server) DeleteTag(
	ctx context.Context,
	req *connect.Request[pb.DeleteTagRequest],
) (*connect.Response[pb.DeleteTagResponse], error) {
	if req.Msg.Id == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "tag id is required")
	}

	db := s.db.WithContext(ctx)

	tag, err := common.FindByID[models.Tag](common.ActiveTags(db), req.Msg.Id, errors.ErrTagNotFound, "tag not found")
	if err != nil {
		return nil, err
	}

	// The foreign key cascades too; deleting the links first keeps it explicit.
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id = ?", tag.ID).Delete(&models.EntryTag{}).Error; err != nil {
			return err
		}

		return tx.Delete(&tag).Error
	}); err != nil {
		return nil, errors.InternalCause(err, "failed to delete tag")
	}

	return connect.NewResponse(&pb.DeleteTagResponse{}), nil
}
