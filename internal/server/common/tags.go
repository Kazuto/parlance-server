package common

import (
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

// ActiveTags selects tags through query whose scope isn't deleted. Tags of a deleted
// scope stay, hidden, until it is restored.
func ActiveTags(query *gorm.DB) *gorm.DB {
	return query.Model(&models.Tag{}).Where("tags.scope_id IN (?)", activeScopeIDs(query))
}

// PreloadTags loads the entries' active tags by name along with them
func PreloadTags(query *gorm.DB) *gorm.DB {
	scopeIDs := activeScopeIDs(query)

	return query.Preload("Tags", func(tx *gorm.DB) *gorm.DB {
		return tx.Where("tags.scope_id IN (?)", scopeIDs).Order("tags.name")
	})
}

// activeScopeIDs is a subquery for the IDs of scopes that aren't deleted. It starts a
// separate statement so it doesn't inherit the conditions of query.
func activeScopeIDs(query *gorm.DB) *gorm.DB {
	return query.Session(&gorm.Session{NewDB: true}).Model(&models.Scope{}).Select("id")
}

// ResolveActiveTags loads the active tags with the IDs, ignoring blanks and duplicates. An
// unknown ID, or one of a deleted scope, is invalid input.
func ResolveActiveTags(db *gorm.DB, ids []string) ([]models.Tag, error) {
	return ResolveByIDs[models.Tag](ActiveTags(db), ids, errors.ErrTagNotFound, "one or more tags do not exist")
}
