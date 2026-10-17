package stats

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// GetStats returns translation statistics
func (s *Server) GetStats(
	ctx context.Context,
	req *connect.Request[pb.GetStatsRequest],
) (*connect.Response[pb.GetStatsResponse], error) {
	// Count total entries
	var totalEntries int64
	if err := s.db.Model(&models.Entry{}).Count(&totalEntries).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Count total locales
	var totalLocales int64
	if err := s.db.Model(&models.Locale{}).Count(&totalLocales).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Calculate total possible translations
	totalPossible := totalEntries * totalLocales

	// Count completed translations (localizations that exist, excluding soft-deleted)
	var completedTranslations int64
	if err := s.db.Model(&models.Localization{}).
		Joins("JOIN entries ON entries.id = localizations.entry_id AND entries.deleted_at IS NULL").
		Joins("JOIN locales ON locales.id = localizations.locale_id AND locales.deleted_at IS NULL").
		Count(&completedTranslations).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Calculate missing translations
	missingTranslations := totalPossible - completedTranslations

	// Count fully translated entries (entries that have translations in all locales)
	var fullyTranslatedEntries int64
	if totalLocales > 0 {
		// Get entries that have exactly totalLocales localizations (excluding soft-deleted)
		var result struct{ Count int64 }
		if err := s.db.Raw(`
			SELECT COUNT(*) as count
			FROM (
				SELECT l.entry_id
				FROM localizations l
				JOIN entries e ON e.id = l.entry_id AND e.deleted_at IS NULL
				JOIN locales loc ON loc.id = l.locale_id AND loc.deleted_at IS NULL
				WHERE l.deleted_at IS NULL
				GROUP BY l.entry_id
				HAVING COUNT(DISTINCT l.locale_id) = ?
			) AS fully_translated
		`, totalLocales).Scan(&result).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		fullyTranslatedEntries = result.Count
	}

	// Get all locales for per-locale stats
	var locales []models.Locale
	if err := s.db.Find(&locales).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Build per-locale stats
	byLocale := make([]*pb.LocaleStats, 0, len(locales))
	for _, locale := range locales {
		// Count completed translations for this locale (excluding soft-deleted entries)
		var completed int64
		if err := s.db.Model(&models.Localization{}).
			Joins("JOIN entries ON entries.id = localizations.entry_id AND entries.deleted_at IS NULL").
			Where("localizations.locale_id = ?", locale.ID).
			Count(&completed).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		missing := totalEntries - completed
		var percentage int32
		if totalEntries > 0 {
			percentage = int32((completed * 100) / totalEntries)
		}

		byLocale = append(byLocale, &pb.LocaleStats{
			LocaleId:   locale.ID,
			LocaleCode: locale.Code,
			Completed:  int32(completed),
			Missing:    int32(missing),
			Percentage: percentage,
		})
	}

	return connect.NewResponse(&pb.GetStatsResponse{
		TotalEntries:              int32(totalEntries),
		TotalLocales:              int32(totalLocales),
		TotalPossibleTranslations: int32(totalPossible),
		CompletedTranslations:     int32(completedTranslations),
		MissingTranslations:       int32(missingTranslations),
		ByLocale:                  byLocale,
		FullyTranslatedEntries:    int32(fullyTranslatedEntries),
	}), nil
}
