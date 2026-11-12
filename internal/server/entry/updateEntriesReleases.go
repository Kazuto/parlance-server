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

// UpdateEntriesReleases adds several entries to pending releases and removes them from
// others in one transaction
func (s *Server) UpdateEntriesReleases(
	ctx context.Context,
	req *connect.Request[pb.UpdateEntriesReleasesRequest],
) (*connect.Response[pb.UpdateEntriesReleasesResponse], error) {
	entryIDs := common.UniqueIDs(req.Msg.EntryIds)
	addIDs := common.UniqueIDs(req.Msg.AddReleaseIds)
	removeIDs := common.UniqueIDs(req.Msg.RemoveReleaseIds)

	if len(entryIDs) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "at least one entry id is required")
	}

	if len(entryIDs) > maxBulkEntries {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "too many entries in one request")
	}

	if len(addIDs) == 0 && len(removeIDs) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "no releases to add or remove")
	}

	for _, id := range addIDs {
		if slices.Contains(removeIDs, id) {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "an entry cannot be added to and removed from a release at once")
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

	// Releases that went out keep what they shipped.
	added, err := common.ResolvePendingReleases(db, addIDs)
	if err != nil {
		return nil, err
	}

	if _, err := common.ResolvePendingReleases(db, removeIDs); err != nil {
		return nil, err
	}

	userID := middleware.UserIDPtr(ctx)

	if err := db.Transaction(func(tx *gorm.DB) error {
		if len(removeIDs) > 0 {
			if err := tx.Where("entry_id IN ? AND release_id IN ?", entryIDs, removeIDs).
				Delete(&models.ReleaseEntry{}).Error; err != nil {
				return err
			}
		}

		if len(added) > 0 {
			links := make([]models.ReleaseEntry, 0, len(entryIDs)*len(added))

			for _, entryID := range entryIDs {
				for _, release := range added {
					links = append(links, models.ReleaseEntry{ReleaseID: release.ID, EntryID: entryID, CreatedBy: userID})
				}
			}

			// Links the entries already have are skipped by the primary key.
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
				CreateInBatches(&links, 500).Error; err != nil {
				return err
			}
		}

		return tx.Model(&models.Entry{}).Where("id IN ?", entryIDs).
			Updates(map[string]any{"updated_by": userID, "updated_at": time.Now()}).Error
	}); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update entry releases")
	}

	return connect.NewResponse(&pb.UpdateEntriesReleasesResponse{}), nil
}
