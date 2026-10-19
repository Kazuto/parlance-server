package entry

import (
	"context"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxBulkEntries = 1000

// UpdateEntriesScopes adds and removes scopes on several entries in one transaction
func (s *Server) UpdateEntriesScopes(
	ctx context.Context,
	req *connect.Request[pb.UpdateEntriesScopesRequest],
) (*connect.Response[pb.UpdateEntriesScopesResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateEntry); err != nil {
		return nil, err
	}

	entryIDs := uniqueIDs(req.Msg.EntryIds)
	addIDs := uniqueIDs(req.Msg.AddScopeIds)
	removeIDs := uniqueIDs(req.Msg.RemoveScopeIds)

	if len(entryIDs) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "at least one entry id is required")
	}
	if len(entryIDs) > maxBulkEntries {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "too many entries in one request")
	}
	if len(addIDs) == 0 && len(removeIDs) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "no scopes to add or remove")
	}
	for _, id := range addIDs {
		if slices.Contains(removeIDs, id) {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "a scope cannot be added and removed at once")
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

	scopes, err := resolveScopes(db, addIDs)
	if err != nil {
		return nil, err
	}

	var userID *string
	if id, _ := ctx.Value(middleware.ContextKeyUserID).(string); id != "" {
		userID = &id
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if len(removeIDs) > 0 {
			if err := tx.Where("entry_id IN ? AND scope_id IN ?", entryIDs, removeIDs).
				Delete(&models.EntryScope{}).Error; err != nil {
				return err
			}
		}

		if len(scopes) > 0 {
			var existing []models.EntryScope
			if err := tx.Select("entry_id", "scope_id").
				Where("entry_id IN ? AND scope_id IN ?", entryIDs, addIDs).
				Find(&existing).Error; err != nil {
				return err
			}
			assigned := map[[2]string]bool{}
			for _, link := range existing {
				assigned[[2]string{link.EntryID, link.ScopeID}] = true
			}

			var links []models.EntryScope
			for _, entryID := range entryIDs {
				for _, scope := range scopes {
					if !assigned[[2]string{entryID, scope.ID}] {
						links = append(links, models.EntryScope{EntryID: entryID, ScopeID: scope.ID, CreatedBy: userID})
					}
				}
			}
			if len(links) > 0 {
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
					Omit("Entry", "Scope").
					CreateInBatches(&links, 500).Error; err != nil {
					return err
				}
			}
		}

		return tx.Model(&models.Entry{}).Where("id IN ?", entryIDs).
			Updates(map[string]any{"updated_by": userID, "updated_at": time.Now()}).Error
	}); err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to update entry scopes")
	}

	return connect.NewResponse(&pb.UpdateEntriesScopesResponse{}), nil
}

func uniqueIDs(ids []string) []string {
	seen := map[string]bool{}
	var unique []string
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	return unique
}
