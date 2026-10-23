package importservice

import (
	"context"
	stderrors "errors"
	"log"

	"connectrpc.com/connect"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
)

// importPlan lists the writes an import makes, computed from the existing rows.
type importPlan struct {
	entriesToCreate       []models.Entry
	localizationsToCreate []models.Localization
	localizationsToUpdate []models.Localization
	entriesToAddScopes    []*models.Entry
}

// processImport handles the common import logic for all formats. All reads and writes,
// including the REPLACE_ALL deletion, run in one transaction, so a failed import changes nothing.
func (s *Server) processImport(
	ctx context.Context,
	localeID string,
	scopeIDs []string,
	translations map[string]string,
	mode pb.ImportMode,
) (*pb.ImportResult, error) {
	userID := middleware.UserID(ctx)
	if userID == "" {
		return nil, errors.Unauthenticated(errors.ErrUnauthenticated, "user not authenticated")
	}

	result := &pb.ImportResult{Errors: []*pb.ImportError{}}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Locale{}, "id = ?", localeID).Error; err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.NotFound(errors.ErrLocaleNotFound, "locale not found")
			}

			return err
		}

		scopes, err := common.ResolveByIDs[models.Scope](tx, scopeIDs, errors.ErrScopeNotFound, "one or more scopes do not exist")
		if err != nil {
			return err
		}

		if mode == pb.ImportMode_IMPORT_MODE_REPLACE_ALL && len(scopeIDs) > 0 {
			if err := deleteScopeLocalizations(tx, userID, localeID, scopeIDs); err != nil {
				return err
			}
		}

		plan, err := planImport(tx, result, userID, localeID, translations, mode, len(scopes) > 0)
		if err != nil {
			return err
		}

		return applyImport(tx, result, plan, scopes, userID, localeID, translations)
	})
	if err != nil {
		if connect.CodeOf(err) != connect.CodeUnknown {
			return nil, err
		}

		log.Printf("import into locale %s failed: %v", localeID, err)

		return nil, errors.Internal(errors.ErrInternal, "import failed")
	}

	return result, nil
}

// deleteScopeLocalizations removes the locale's translations of all entries in the scopes,
// one row at a time so each deletion is recorded in the history with its author.
func deleteScopeLocalizations(tx *gorm.DB, userID, localeID string, scopeIDs []string) error {
	var localizations []models.Localization
	if err := tx.
		Where("locale_id = ?", localeID).
		Where("entry_id IN (?)", tx.Table("entry_scopes").Select("entry_id").Where("scope_id IN ?", scopeIDs)).
		Find(&localizations).Error; err != nil {
		return err
	}

	for i := range localizations {
		localizations[i].UpdatedBy = &userID // credited in the deletion's history row
		if err := tx.Delete(&localizations[i]).Error; err != nil {
			return err
		}
	}

	return nil
}

// planImport sorts the translations into creates, updates and skips according to the mode
func planImport(
	tx *gorm.DB,
	result *pb.ImportResult,
	userID, localeID string,
	translations map[string]string,
	mode pb.ImportMode,
	hasScopes bool,
) (*importPlan, error) {
	keys := make([]string, 0, len(translations))
	for key := range translations {
		keys = append(keys, key)
	}

	var existingEntries []models.Entry
	if err := tx.Where("key IN ?", keys).Find(&existingEntries).Error; err != nil {
		return nil, err
	}

	entryMap := make(map[string]*models.Entry, len(existingEntries))
	entryIDs := make([]string, 0, len(existingEntries))

	for i := range existingEntries {
		entryMap[existingEntries[i].Key] = &existingEntries[i]
		entryIDs = append(entryIDs, existingEntries[i].ID)
	}

	var existingLocalizations []models.Localization
	if len(entryIDs) > 0 {
		if err := tx.Where("entry_id IN ? AND locale_id = ?", entryIDs, localeID).
			Find(&existingLocalizations).Error; err != nil {
			return nil, err
		}
	}

	localizationMap := make(map[string]*models.Localization, len(existingLocalizations))
	for i := range existingLocalizations {
		localizationMap[existingLocalizations[i].EntryID] = &existingLocalizations[i]
	}

	plan := &importPlan{}

	for key, value := range translations {
		entry, entryExists := entryMap[key]

		if mode == pb.ImportMode_IMPORT_MODE_CREATE_ONLY && entryExists {
			result.EntriesSkipped++

			continue
		}

		if mode == pb.ImportMode_IMPORT_MODE_UPDATE_ONLY && !entryExists {
			result.EntriesSkipped++

			continue
		}

		// New entries get their localization once they have an ID
		if !entryExists {
			plan.entriesToCreate = append(plan.entriesToCreate, models.Entry{Key: key, CreatedBy: &userID})

			continue
		}

		if hasScopes {
			plan.entriesToAddScopes = append(plan.entriesToAddScopes, entry)
		}

		if localization, ok := localizationMap[entry.ID]; ok {
			localization.Translation = value
			localization.UpdatedBy = &userID
			plan.localizationsToUpdate = append(plan.localizationsToUpdate, *localization)
			result.EntriesUpdated++
		} else {
			plan.localizationsToCreate = append(plan.localizationsToCreate, models.Localization{
				EntryID:     entry.ID,
				LocaleID:    localeID,
				Translation: value,
				CreatedBy:   &userID,
			})
			result.EntriesCreated++
		}
	}

	return plan, nil
}

// applyImport writes the planned changes
func applyImport(
	tx *gorm.DB,
	result *pb.ImportResult,
	plan *importPlan,
	scopes []models.Scope,
	userID, localeID string,
	translations map[string]string,
) error {
	if len(plan.entriesToCreate) > 0 {
		if err := tx.Create(&plan.entriesToCreate).Error; err != nil {
			return err
		}

		for i := range plan.entriesToCreate {
			entry := &plan.entriesToCreate[i]
			if len(scopes) > 0 {
				if err := tx.Model(entry).Association("Scopes").Append(&scopes); err != nil {
					return err
				}
			}

			plan.localizationsToCreate = append(plan.localizationsToCreate, models.Localization{
				EntryID:     entry.ID,
				LocaleID:    localeID,
				Translation: translations[entry.Key],
				CreatedBy:   &userID,
			})
			result.EntriesCreated++
		}
	}

	for _, entry := range plan.entriesToAddScopes {
		if err := tx.Model(entry).Association("Scopes").Append(&scopes); err != nil {
			return err
		}
	}

	if len(plan.localizationsToCreate) > 0 {
		if err := tx.Create(&plan.localizationsToCreate).Error; err != nil {
			return err
		}
	}

	for i := range plan.localizationsToUpdate {
		if err := tx.Save(&plan.localizationsToUpdate[i]).Error; err != nil {
			return err
		}
	}

	return nil
}

// startImport authorizes the import and checks its content, returning the mode with
// CREATE_OR_UPDATE as default. REPLACE_ALL also needs delete_entry on top of the rule for
// imports, as it removes existing localizations.
func (s *Server) startImport(ctx context.Context, mode pb.ImportMode, content []byte) (pb.ImportMode, error) {
	if mode == pb.ImportMode_IMPORT_MODE_REPLACE_ALL {
		if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteEntry); err != nil {
			return mode, err
		}
	}

	if len(content) == 0 {
		return mode, errors.InvalidInput(errors.ErrInvalidInput, "file_content is required")
	}

	if mode == pb.ImportMode_IMPORT_MODE_UNSPECIFIED {
		mode = pb.ImportMode_IMPORT_MODE_CREATE_OR_UPDATE
	}

	return mode, nil
}

// requireLocaleID checks the locale a single-file import goes into
func requireLocaleID(localeID string) error {
	if localeID == "" {
		return errors.InvalidInput(errors.ErrInvalidInput, "locale_id is required")
	}

	return nil
}
