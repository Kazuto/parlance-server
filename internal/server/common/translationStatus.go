package common

import (
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

// TranslationRows names a table of texts per locale and how its rows belong to the
// listed records, for FilterTranslationStatus.
type TranslationRows struct {
	// The table, e.g. localizations
	Table string
	// Its column pointing at the listed record, e.g. entry_id
	Owner string
	// The listed record's id, e.g. entries.id
	OwnerID string
	// Whether its rows can be outdated
	Outdated bool
}

var (
	// Localizations are an entry's translations.
	Localizations = TranslationRows{Table: "localizations", Owner: "entry_id", OwnerID: "entries.id", Outdated: true}
	// Definitions are a glossary term's translations.
	Definitions = TranslationRows{Table: "definitions", Owner: "terminology_id", OwnerID: "terminologies.id"}
)

// FilterTranslationStatus keeps records by their translations in the given locales (all
// active ones when none are given): "missing" one, "complete" in all, one in "review"
// as a draft, or one "outdated" when the rows can be. Blank and deleted translations
// are missing.
func FilterTranslationStatus(query *gorm.DB, rows TranslationRows, status string, localeIDs []string) *gorm.DB {
	// Start a separate statement so the correlated subquery doesn't inherit the filters.
	targets := query.Session(&gorm.Session{NewDB: true}).Model(&models.Locale{}).Select("1")
	if len(localeIDs) > 0 {
		targets = targets.Where("locales.id IN ?", localeIDs)
	}

	// The record's non-blank translation in the locale
	translation := "SELECT 1 FROM " + rows.Table + " WHERE " + rows.Table + "." + rows.Owner + " = " + rows.OwnerID +
		" AND " + rows.Table + ".locale_id = locales.id AND " + rows.Table + ".deleted_at IS NULL" +
		" AND TRIM(" + rows.Table + ".translation) <> ''"

	switch status {
	case "review":
		return query.Where("EXISTS (?)", targets.Where("EXISTS ("+translation+" AND "+rows.Table+".status = ?)", models.StatusDraft))
	case "outdated":
		if !rows.Outdated {
			return query.Where("1 = 0")
		}

		return query.Where("EXISTS (?)", targets.Where("EXISTS ("+translation+" AND "+rows.Table+".outdated_at IS NOT NULL)"))
	}

	targets = targets.Where("NOT EXISTS (" + translation + ")")
	if status == "missing" {
		return query.Where("EXISTS (?)", targets)
	}

	return query.Where("NOT EXISTS (?)", targets)
}
