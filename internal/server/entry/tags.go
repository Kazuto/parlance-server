package entry

import (
	"slices"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// checkTagsInScopes rejects tags of scopes the entry isn't in
func checkTagsInScopes(tags []models.Tag, scopeIDs []string) error {
	for _, tag := range tags {
		if !slices.Contains(scopeIDs, tag.ScopeID) {
			return errors.InvalidInput(errors.ErrTagOutsideScopes, "tags must belong to the entry's scopes")
		}
	}

	return nil
}

// replaceTags sets the entry's tags to tags, keeping the links it already has
func replaceTags(tx *gorm.DB, entryID string, tags []models.Tag, userID *string) error {
	tagIDs := make([]string, len(tags))
	for i, tag := range tags {
		tagIDs[i] = tag.ID
	}

	stale := tx.Where("entry_id = ?", entryID)
	if len(tagIDs) > 0 {
		stale = stale.Where("tag_id NOT IN ?", tagIDs)
	}

	if err := stale.Delete(&models.EntryTag{}).Error; err != nil {
		return err
	}

	if len(tags) == 0 {
		return nil
	}

	links := make([]models.EntryTag, len(tags))
	for i, tag := range tags {
		links[i] = models.EntryTag{EntryID: entryID, TagID: tag.ID, ScopeID: tag.ScopeID, CreatedBy: userID}
	}

	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&links).Error
}

// dropTagsOutside removes the entry's tags of scopes other than scopeIDs. The database
// does it too when an entry leaves a scope; doing it here keeps it explicit.
func dropTagsOutside(tx *gorm.DB, entryID string, scopeIDs []string) error {
	query := tx.Where("entry_id = ?", entryID)
	if len(scopeIDs) > 0 {
		query = query.Where("scope_id NOT IN ?", scopeIDs)
	}

	return query.Delete(&models.EntryTag{}).Error
}

// scopeIDsOf returns the IDs of the scopes
func scopeIDsOf(scopes []models.Scope) []string {
	ids := make([]string, len(scopes))
	for i, scope := range scopes {
		ids[i] = scope.ID
	}

	return ids
}
