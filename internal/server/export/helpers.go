package export

import (
	stderrors "errors"
	"strings"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
)

// exportedEntry is an entry with its translation in the exported locale, if it has one
type exportedEntry struct {
	Key         string
	Description string
	Translation string
	Translated  bool
}

// findLocale loads the locale with the code
func findLocale(db *gorm.DB, code string) (models.Locale, error) {
	var locale models.Locale
	if code == "" {
		return locale, errors.InvalidInput(errors.ErrInvalidInput, "locale code is required")
	}

	if err := db.Where("code = ?", code).First(&locale).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return locale, errors.NotFound(errors.ErrLocaleNotFound, "locale not found")
		}

		return locale, errors.InternalCause(err, "failed to load locale")
	}

	return locale, nil
}

// loadEntries loads the entries in any of the scopes (all entries without scopes), ordered by
// key, with their translation in the locale
func loadEntries(db *gorm.DB, localeID string, scopeIDs []string) ([]exportedEntry, error) {
	var entries []models.Entry

	query := common.FilterEntriesByScopes(db.Model(&models.Entry{}), scopeIDs).
		Preload("Localizations", "locale_id = ?", localeID).
		Order("key")
	if err := query.Find(&entries).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to query entries")
	}

	exported := make([]exportedEntry, len(entries))
	for i, entry := range entries {
		exported[i] = exportedEntry{Key: entry.Key, Description: entry.Description}
		if len(entry.Localizations) > 0 {
			exported[i].Translation = entry.Localizations[0].Translation
			exported[i].Translated = true
		}
	}

	return exported, nil
}

// translationMap maps each translated entry's key to its translation
func translationMap(entries []exportedEntry) map[string]string {
	translations := make(map[string]string, len(entries))

	for _, entry := range entries {
		if entry.Translated {
			translations[entry.Key] = entry.Translation
		}
	}

	return translations
}

// scopeNames joins the sorted names of the scopes with "-", or returns "" without scopes
func scopeNames(db *gorm.DB, scopeIDs []string) string {
	if len(scopeIDs) == 0 {
		return ""
	}

	var names []string
	if err := db.Model(&models.Scope{}).Where("id IN ?", scopeIDs).Order("name").Pluck("name", &names).Error; err != nil {
		return ""
	}

	return strings.Join(names, "-")
}

// exportFilename prefixes the locale with the sorted scope names, e.g. "backend-shared_de.json".
func exportFilename(db *gorm.DB, scopeIDs []string, localeCode, extension string) string {
	filename := localeCode + "." + extension
	if names := scopeNames(db, scopeIDs); names != "" {
		return names + "_" + filename
	}

	return filename
}

// phpString quotes text as a PHP single-quoted string literal
func phpString(text string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(text) + "'"
}
