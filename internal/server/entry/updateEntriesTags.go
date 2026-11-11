package entry

import (
	"context"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UpdateEntriesTags adds and removes tags on several entries in one transaction. An added
// tag brings its scope to entries not in it yet.
func (s *Server) UpdateEntriesTags(
	ctx context.Context,
	req *connect.Request[pb.UpdateEntriesTagsRequest],
) (*connect.Response[pb.UpdateEntriesTagsResponse], error) {
	entryIDs := common.UniqueIDs(req.Msg.EntryIds)
	addIDs := common.UniqueIDs(req.Msg.AddTagIds)
	removeIDs := common.UniqueIDs(req.Msg.RemoveTagIds)

	if len(entryIDs) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "at least one entry id is required")
	}

	if len(entryIDs) > maxBulkEntries {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "too many entries in one request")
	}

	if len(addIDs) == 0 && len(removeIDs) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "no tags to add or remove")
	}

	for _, id := range addIDs {
		if slices.Contains(removeIDs, id) {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "a tag cannot be added and removed at once")
		}
	}

	db := s.db.WithContext(ctx)

	var count int64

	if err := db.Model(&models.Entry{}).Where("id IN ?", entryIDs).Count(&count).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to find entries")
	}

	if int(count) != len(entryIDs) {
		return nil, errors.NotFound(errors.ErrEntryNotFound, "one or more entries do not exist")
	}

	tags, err := common.ResolveActiveTags(db, addIDs)
	if err != nil {
		return nil, err
	}

	userID := middleware.UserIDPtr(ctx)

	if err := db.Transaction(func(tx *gorm.DB) error {
		if len(removeIDs) > 0 {
			if err := tx.Where("entry_id IN ? AND tag_id IN ?", entryIDs, removeIDs).
				Delete(&models.EntryTag{}).Error; err != nil {
				return err
			}
		}

		if len(tags) > 0 {
			var scopeLinks []models.EntryScope

			var tagLinks []models.EntryTag

			linked := map[[2]string]bool{}

			for _, entryID := range entryIDs {
				for _, tag := range tags {
					if !linked[[2]string{entryID, tag.ScopeID}] {
						linked[[2]string{entryID, tag.ScopeID}] = true
						scopeLinks = append(scopeLinks, models.EntryScope{EntryID: entryID, ScopeID: tag.ScopeID, CreatedBy: userID})
					}
					tagLinks = append(tagLinks, models.EntryTag{EntryID: entryID, TagID: tag.ID, ScopeID: tag.ScopeID, CreatedBy: userID})
				}
			}

			// Links the entries already have are skipped by the unique indexes.
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
				Omit("Entry", "Scope").
				CreateInBatches(&scopeLinks, 500).Error; err != nil {
				return err
			}

			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
				CreateInBatches(&tagLinks, 500).Error; err != nil {
				return err
			}
		}

		return tx.Model(&models.Entry{}).Where("id IN ?", entryIDs).
			Updates(map[string]any{"updated_by": userID, "updated_at": time.Now()}).Error
	}); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update entry tags")
	}

	return connect.NewResponse(&pb.UpdateEntriesTagsResponse{}), nil
}
