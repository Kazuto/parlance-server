package entry

import (
	"context"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/qa"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/translate"
)

// entriesToCheck returns the entries with a translation in one of the locales (all
// but the default when none are given) that the QA checks flag against the default
// text, as the editor does. The checks run in Go, so this reads the translations
// once rather than filtering in SQL.
func (s *Server) entriesToCheck(ctx context.Context, localeIDs []string) ([]string, error) {
	db := s.db.WithContext(ctx)

	defaultLocale, err := common.DefaultLocale(db)
	if err != nil || defaultLocale == nil {
		return []string{}, err
	}

	localeQuery := db.Where("id <> ?", defaultLocale.ID)
	if len(localeIDs) > 0 {
		localeQuery = localeQuery.Where("id IN ?", localeIDs)
	}

	var locales []models.Locale
	if err := localeQuery.Find(&locales).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load locales")
	}

	if len(locales) == 0 {
		return []string{}, nil
	}

	targets := make(map[string]qa.Target, len(locales))
	ids := make([]string, len(locales))

	for i, locale := range locales {
		targets[locale.ID] = qa.Target{Code: locale.Code, Formality: translate.Formality(locale.Formality)}
		ids[i] = locale.ID
	}

	var rows []struct {
		EntryID     string
		LocaleID    string
		Translation string
		Source      string
	}

	err = db.Table("localizations").
		Select("localizations.entry_id, localizations.locale_id, localizations.translation, source.translation AS source").
		Joins("JOIN localizations source ON source.entry_id = localizations.entry_id AND source.locale_id = ? AND source.deleted_at IS NULL", defaultLocale.ID).
		Where("localizations.deleted_at IS NULL AND localizations.locale_id IN ? AND TRIM(localizations.translation) <> ''", ids).
		Scan(&rows).Error
	if err != nil {
		return nil, errors.InternalCause(err, "failed to load translations")
	}

	flagged := map[string]bool{}
	entryIDs := []string{}

	for _, row := range rows {
		if flagged[row.EntryID] || len(qa.Check(row.Source, row.Translation, targets[row.LocaleID])) == 0 {
			continue
		}

		flagged[row.EntryID] = true

		entryIDs = append(entryIDs, row.EntryID)
	}

	return entryIDs, nil
}
