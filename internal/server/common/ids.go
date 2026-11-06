package common

import (
	"github.com/kazuto/parlance-server/internal/errors"
	"gorm.io/gorm"
)

// UniqueIDs drops blank and duplicate IDs, keeping the order they first appear in
func UniqueIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	unique := make([]string, 0, len(ids))

	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true

			unique = append(unique, id)
		}
	}

	return unique
}

// ResolveByIDs loads the rows with the IDs, ignoring blanks and duplicates. An unknown ID is
// invalid input with the code and message.
func ResolveByIDs[T any](query *gorm.DB, ids []string, invalidCode, invalidMessage string) ([]T, error) {
	ids = UniqueIDs(ids)
	rows := []T{}

	if len(ids) == 0 {
		return rows, nil
	}

	if err := query.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load records")
	}

	if len(rows) != len(ids) {
		return nil, errors.InvalidInput(invalidCode, invalidMessage)
	}

	return rows, nil
}

// FilterEntriesByScopes keeps entries in any of the scopes; without scopes it keeps all. A
// subquery instead of a join keeps entries in several of the scopes from appearing twice.
func FilterEntriesByScopes(query *gorm.DB, scopeIDs []string) *gorm.DB {
	if len(scopeIDs) == 0 {
		return query
	}

	return query.Where("entries.id IN (SELECT entry_id FROM entry_scopes WHERE scope_id IN ?)", scopeIDs)
}
