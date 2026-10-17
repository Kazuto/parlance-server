package export

import (
	"strings"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

// setNestedKey sets a value in a nested map using dot notation
// e.g., "user.profile.name" -> {"user": {"profile": {"name": value}}}
func setNestedKey(m map[string]interface{}, key string, value string) {
	parts := strings.Split(key, ".")

	// If no dot notation, just set directly
	if len(parts) == 1 {
		m[key] = value
		return
	}

	// Navigate/create nested maps
	current := m
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]

		if _, exists := current[part]; !exists {
			current[part] = make(map[string]interface{})
		}

		// Type assertion to continue navigating
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			// If the key already exists as a non-map value, override it
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}

	// Set the final value
	current[parts[len(parts)-1]] = value
}

// requestedScopeIDs drops blank and duplicate scope ids.
func requestedScopeIDs(scopeIDs []string) []string {
	seen := make(map[string]bool)
	ids := make([]string, 0, len(scopeIDs))
	for _, id := range scopeIDs {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// filterByScopes limits entries to those in any of the scopes. A subquery
// keeps entries in several matching scopes from being returned twice.
func filterByScopes(db *gorm.DB, query *gorm.DB, scopeIDs []string) *gorm.DB {
	if len(scopeIDs) == 0 {
		return query
	}
	return query.Where("entries.id IN (?)",
		db.Table("entry_scopes").Select("entry_id").Where("scope_id IN ?", scopeIDs))
}

// exportFilename prefixes the locale with the sorted scope names, e.g. "backend-shared_de.json".
func exportFilename(db *gorm.DB, scopeIDs []string, localeCode, extension string) string {
	filename := localeCode + "." + extension
	if len(scopeIDs) == 0 {
		return filename
	}
	var names []string
	if err := db.Model(&models.Scope{}).Where("id IN ?", scopeIDs).Order("name").Pluck("name", &names).Error; err != nil || len(names) == 0 {
		return filename
	}
	return strings.Join(names, "-") + "_" + filename
}
