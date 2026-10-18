package importservice

import (
	"context"
	"fmt"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

// processImport handles the common import logic for all formats
func (s *Server) processImport(
	ctx context.Context,
	localeID string,
	scopeIDs []string,
	translations map[string]string,
	mode pb.ImportMode,
) (*pb.ImportResult, error) {
	result := &pb.ImportResult{
		Errors: []*pb.ImportError{},
	}

	// Get user ID from context
	userID, _ := ctx.Value(middleware.ContextKeyUserID).(string)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	// Verify locale exists
	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", localeID).Error; err != nil {
		return nil, fmt.Errorf("locale not found: %w", err)
	}

	// Verify scopes exist
	var scopes []models.Scope
	if len(scopeIDs) > 0 {
		if err := s.db.Where("id IN ?", scopeIDs).Find(&scopes).Error; err != nil {
			return nil, fmt.Errorf("failed to find scopes: %w", err)
		}
	}

	// Handle REPLACE_ALL mode
	if mode == pb.ImportMode_IMPORT_MODE_REPLACE_ALL && len(scopeIDs) > 0 {
		// Delete all localizations for entries in these scopes
		var entryIDs []string
		s.db.Model(&models.Entry{}).
			Joins("JOIN entry_scopes ON entry_scopes.entry_id = entries.id").
			Where("entry_scopes.scope_id IN ?", scopeIDs).
			Pluck("entries.id", &entryIDs)

		if len(entryIDs) > 0 {
			s.db.Where("entry_id IN ? AND locale_id = ?", entryIDs, localeID).
				Delete(&models.Localization{})
		}
	}

	// Batch fetch existing entries for all keys
	keys := make([]string, 0, len(translations))
	for key := range translations {
		keys = append(keys, key)
	}

	var existingEntries []models.Entry
	if err := s.db.Where("key IN ?", keys).Find(&existingEntries).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch existing entries: %w", err)
	}

	// Create a map of existing entries by key
	entryMap := make(map[string]*models.Entry)
	entryIDs := make([]string, 0, len(existingEntries))
	for i := range existingEntries {
		entryMap[existingEntries[i].Key] = &existingEntries[i]
		entryIDs = append(entryIDs, existingEntries[i].ID)
	}

	// Batch fetch existing localizations
	var existingLocalizations []models.Localization
	if len(entryIDs) > 0 {
		if err := s.db.Where("entry_id IN ? AND locale_id = ?", entryIDs, localeID).
			Find(&existingLocalizations).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch existing localizations: %w", err)
		}
	}

	// Create a map of existing localizations by entry_id
	localizationMap := make(map[string]*models.Localization)
	for i := range existingLocalizations {
		localizationMap[existingLocalizations[i].EntryID] = &existingLocalizations[i]
	}

	// Process translations in batches
	entriesToCreate := []models.Entry{}
	localizationsToCreate := []models.Localization{}
	localizationsToUpdate := []models.Localization{}
	entriesToAddScopes := []*models.Entry{}

	for key, value := range translations {
		entry, entryExists := entryMap[key]

		// Handle mode-based skipping
		if mode == pb.ImportMode_IMPORT_MODE_CREATE_ONLY && entryExists {
			result.EntriesSkipped++
			continue
		}
		if mode == pb.ImportMode_IMPORT_MODE_UPDATE_ONLY && !entryExists {
			result.EntriesSkipped++
			continue
		}

		// Create entry if it doesn't exist
		if !entryExists {
			newEntry := models.Entry{
				Key:       key,
				CreatedBy: &userID,
			}
			entriesToCreate = append(entriesToCreate, newEntry)
			// We'll need to handle localization after entry is created
			continue
		}

		// Add scopes if needed
		if len(scopes) > 0 {
			entriesToAddScopes = append(entriesToAddScopes, entry)
		}

		// Check if localization exists
		localization, localizationExists := localizationMap[entry.ID]

		if localizationExists {
			// Update existing localization
			localization.Translation = value
			localization.UpdatedBy = &userID
			localizationsToUpdate = append(localizationsToUpdate, *localization)
			result.EntriesUpdated++
		} else {
			// Create new localization
			newLocalization := models.Localization{
				EntryID:     entry.ID,
				LocaleID:    localeID,
				Translation: value,
				CreatedBy:   &userID,
			}
			localizationsToCreate = append(localizationsToCreate, newLocalization)
			result.EntriesCreated++
		}
	}

	// Execute batch operations in a transaction
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		// Batch create entries
		if len(entriesToCreate) > 0 {
			if err := tx.Create(&entriesToCreate).Error; err != nil {
				return fmt.Errorf("failed to create entries: %w", err)
			}

			// Now create localizations for the new entries
			for i := range entriesToCreate {
				entry := &entriesToCreate[i]
				value := translations[entry.Key]

				// Add scopes if needed
				if len(scopes) > 0 {
					if err := tx.Model(entry).Association("Scopes").Append(&scopes); err != nil {
						return fmt.Errorf("failed to assign scopes to new entry: %w", err)
					}
				}

				newLocalization := models.Localization{
					EntryID:     entry.ID,
					LocaleID:    localeID,
					Translation: value,
					CreatedBy:   &userID,
				}
				localizationsToCreate = append(localizationsToCreate, newLocalization)
				result.EntriesCreated++
			}
		}

		// Batch add scopes to existing entries
		for _, entry := range entriesToAddScopes {
			if err := tx.Model(entry).Association("Scopes").Append(&scopes); err != nil {
				return fmt.Errorf("failed to assign scopes: %w", err)
			}
		}

		// Batch create localizations
		if len(localizationsToCreate) > 0 {
			if err := tx.Create(&localizationsToCreate).Error; err != nil {
				return fmt.Errorf("failed to create localizations: %w", err)
			}
		}

		// Batch update localizations
		for i := range localizationsToUpdate {
			if err := tx.Save(&localizationsToUpdate[i]).Error; err != nil {
				return fmt.Errorf("failed to update localization: %w", err)
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}
