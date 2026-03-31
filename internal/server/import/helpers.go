package importservice

import (
	"context"
	"fmt"

	"github.com/kazuto/parlance-server/internal/models"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
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
	userID, _ := ctx.Value("user_id").(string)
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
	if mode == pb.ImportMode_REPLACE_ALL && len(scopeIDs) > 0 {
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

	// Process each translation
	for key, value := range translations {
		if err := s.processTranslation(ctx, key, value, localeID, scopeIDs, userID, mode, result); err != nil {
			result.EntriesFailed++
			result.Errors = append(result.Errors, &pb.ImportError{
				Key:     key,
				Message: err.Error(),
			})
		}
	}

	return result, nil
}

// processTranslation handles importing a single translation
func (s *Server) processTranslation(
	ctx context.Context,
	key string,
	value string,
	localeID string,
	scopeIDs []string,
	userID string,
	mode pb.ImportMode,
	result *pb.ImportResult,
) error {
	// Find or create entry
	var entry models.Entry
	err := s.db.Where("key = ?", key).First(&entry).Error

	entryExists := err == nil

	// Handle CREATE_ONLY mode
	if mode == pb.ImportMode_CREATE_ONLY && entryExists {
		result.EntriesSkipped++
		return nil
	}

	// Handle UPDATE_ONLY mode
	if mode == pb.ImportMode_UPDATE_ONLY && !entryExists {
		result.EntriesSkipped++
		return nil
	}

	// Create entry if it doesn't exist
	if !entryExists {
		entry = models.Entry{
			Key:       key,
			CreatedBy: &userID,
		}
		if err := s.db.Create(&entry).Error; err != nil {
			return fmt.Errorf("failed to create entry: %w", err)
		}

		// Associate with scopes
		if len(scopeIDs) > 0 {
			var scopes []models.Scope
			s.db.Where("id IN ?", scopeIDs).Find(&scopes)
			s.db.Model(&entry).Association("Scopes").Append(&scopes)
		}
	}

	// Find or create localization
	var localization models.Localization
	err = s.db.Where("entry_id = ? AND locale_id = ?", entry.ID, localeID).
		First(&localization).Error

	localizationExists := err == nil

	if localizationExists {
		// Update existing localization
		localization.Translation = value
		localization.UpdatedBy = &userID
		if err := s.db.Save(&localization).Error; err != nil {
			return fmt.Errorf("failed to update localization: %w", err)
		}
		result.EntriesUpdated++
	} else {
		// Create new localization
		localization = models.Localization{
			EntryID:     entry.ID,
			LocaleID:    localeID,
			Translation: value,
			CreatedBy:   &userID,
		}
		if err := s.db.Create(&localization).Error; err != nil {
			return fmt.Errorf("failed to create localization: %w", err)
		}
		result.EntriesCreated++
	}

	return nil
}
