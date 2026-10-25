package importservice

import (
	"context"
	stderrors "errors"
	"log"
	"slices"
	"time"

	"connectrpc.com/connect"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/taskqueue"
	"github.com/kazuto/parlance-server/internal/webhook"
	"gorm.io/gorm"
)

// importPlan lists the writes an import makes, computed from the existing rows.
type importPlan struct {
	entriesToCreate       []models.Entry
	localizationsToCreate []models.Localization
	localizationsToUpdate []models.Localization
	entriesToAddScopes    []*models.Entry
	// changedEntryIDs holds the entries whose translation the import changes, by id.
	changedEntryIDs map[string]bool
	// outdatingEntryIDs holds the existing entries whose default text the import changes
	// or adds, which makes their other translations outdated.
	outdatingEntryIDs []string
}

// importTarget is the locale an import writes to
type importTarget struct {
	localeID string
	// Whether it is the default locale, whose changes outdate the other translations
	source bool
	// The translations REPLACE_ALL deleted before importing, by entry id
	replaced map[string]models.Localization
}

// processImport handles the common import logic for all formats. All reads and writes,
// including the REPLACE_ALL deletion, run in one transaction, so a failed import changes nothing.
func (s *Server) processImport(
	ctx context.Context,
	localeID string,
	scopeIDs []string,
	translations map[string]string,
	options importOptions,
) (*pb.ImportResult, error) {
	userID := middleware.UserID(ctx)
	if userID == "" {
		return nil, errors.Unauthenticated(errors.ErrUnauthenticated, "user not authenticated")
	}

	result := &pb.ImportResult{Errors: []*pb.ImportError{}}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Taken before any change, so the default texts' history rows come after it
		at := time.Now()

		var locale models.Locale
		if err := tx.First(&locale, "id = ?", localeID).Error; err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				return errors.NotFound(errors.ErrLocaleNotFound, "locale not found")
			}

			return err
		}

		scopes, err := common.ResolveByIDs[models.Scope](tx, scopeIDs, errors.ErrScopeNotFound, "one or more scopes do not exist")
		if err != nil {
			return err
		}

		target := importTarget{localeID: localeID, source: locale.IsDefault}

		if options.mode == pb.ImportMode_IMPORT_MODE_REPLACE_ALL && len(scopeIDs) > 0 {
			if target.replaced, err = deleteScopeLocalizations(tx, userID, localeID, scopeIDs); err != nil {
				return err
			}
		}

		plan, err := planImport(tx, result, userID, target, translations, options, len(scopes) > 0)
		if err != nil {
			return err
		}

		if err := applyImport(tx, result, plan, scopes, userID, localeID, translations, options); err != nil {
			return err
		}

		if _, err := common.MarkOutdated(tx, plan.outdatingEntryIDs, localeID, at); err != nil {
			return err
		}

		return emitImported(tx, plan, &userID, options)
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
// one row at a time so each deletion is recorded in the history with its author. Returns
// the deleted translations by entry id.
func deleteScopeLocalizations(tx *gorm.DB, userID, localeID string, scopeIDs []string) (map[string]models.Localization, error) {
	var localizations []models.Localization
	if err := tx.
		Where("locale_id = ?", localeID).
		Where("entry_id IN (?)", tx.Table("entry_scopes").Select("entry_id").Where("scope_id IN ?", scopeIDs)).
		Find(&localizations).Error; err != nil {
		return nil, err
	}

	replaced := make(map[string]models.Localization, len(localizations))

	for i := range localizations {
		localizations[i].UpdatedBy = &userID // credited in the deletion's history row
		if err := tx.Delete(&localizations[i]).Error; err != nil {
			return nil, err
		}

		replaced[localizations[i].EntryID] = localizations[i]
	}

	return replaced, nil
}

// planImport sorts the translations into creates, updates and skips according to the mode
func planImport(
	tx *gorm.DB,
	result *pb.ImportResult,
	userID string,
	target importTarget,
	translations map[string]string,
	options importOptions,
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
		if err := tx.Where("entry_id IN ? AND locale_id = ?", entryIDs, target.localeID).
			Find(&existingLocalizations).Error; err != nil {
			return nil, err
		}
	}

	localizationMap := make(map[string]*models.Localization, len(existingLocalizations))
	for i := range existingLocalizations {
		localizationMap[existingLocalizations[i].EntryID] = &existingLocalizations[i]
	}

	plan := &importPlan{changedEntryIDs: map[string]bool{}}

	for key, value := range translations {
		entry, entryExists := entryMap[key]

		if options.mode == pb.ImportMode_IMPORT_MODE_CREATE_ONLY && entryExists {
			result.EntriesSkipped++

			continue
		}

		if options.mode == pb.ImportMode_IMPORT_MODE_UPDATE_ONLY && !entryExists {
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
			// Unchanged text keeps its review state, unless the import approves it. Changed
			// or approved text is written for the default text as it is now.
			if localization.Translation != value {
				localization.Translation = value
				localization.MarkDraft()
				localization.ClearOutdated()

				plan.changedEntryIDs[entry.ID] = true
				if target.source {
					plan.outdatingEntryIDs = append(plan.outdatingEntryIDs, entry.ID)
				}
			}

			if options.approve {
				localization.Approve(&userID, time.Now())
				localization.ClearOutdated()
			}

			localization.UpdatedBy = &userID
			plan.localizationsToUpdate = append(plan.localizationsToUpdate, *localization)
			result.EntriesUpdated++
		} else {
			localization := models.Localization{
				EntryID:     entry.ID,
				LocaleID:    target.localeID,
				Translation: value,
				CreatedBy:   &userID,
				Review:      options.newReview(&userID),
			}

			// REPLACE_ALL deleted the text before: the same text again stays outdated
			// unless approved, and only a different default text outdates the others.
			previous, replaced := target.replaced[entry.ID]
			unchanged := replaced && previous.Translation == value

			if unchanged && !options.approve {
				localization.OutdatedAt = previous.OutdatedAt
			}

			if target.source && !unchanged {
				plan.outdatingEntryIDs = append(plan.outdatingEntryIDs, entry.ID)
			}

			plan.localizationsToCreate = append(plan.localizationsToCreate, localization)
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
	options importOptions,
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
				Review:      options.newReview(&userID),
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

// emitImported sends the import's new entries to webhooks, and its translations: as
// approved when it approves them, else the new and changed ones as awaiting approval.
// Approved imports also ship like translations approved in the editor.
func emitImported(tx *gorm.DB, plan *importPlan, userID *string, options importOptions) error {
	if len(plan.entriesToCreate) > 0 {
		ids := make([]string, len(plan.entriesToCreate))
		for i := range plan.entriesToCreate {
			ids[i] = plan.entriesToCreate[i].ID
		}

		if err := webhook.Emit(tx, models.WebhookEventEntriesCreated, func() (any, error) {
			return webhook.Entries(tx, ids, userID)
		}); err != nil {
			return err
		}
	}

	imported := slices.Concat(plan.localizationsToCreate, plan.localizationsToUpdate)

	if !options.approve {
		var drafted []string

		for i, localization := range imported {
			if i < len(plan.localizationsToCreate) || plan.changedEntryIDs[localization.EntryID] {
				drafted = append(drafted, localization.ID)
			}
		}

		return webhook.EmitTranslations(tx, models.WebhookEventTranslationsAwaitingApproval, drafted, userID, nil)
	}

	if len(imported) == 0 {
		return nil
	}

	ids := make([]string, len(imported))
	for i := range imported {
		ids[i] = imported[i].ID
	}

	if err := webhook.EmitTranslations(tx, models.WebhookEventTranslationsApproved, ids, userID, nil); err != nil {
		return err
	}

	return taskqueue.Emit(tx, models.EventTranslationsApproved)
}

// startImport authorizes the import and checks its content, returning its options with
// CREATE_OR_UPDATE as the default mode. On top of the rule for imports, REPLACE_ALL needs
// delete_entry, as it removes existing localizations, and approving needs review_entry.
func (s *Server) startImport(ctx context.Context, mode pb.ImportMode, approve bool, content []byte) (importOptions, error) {
	options := importOptions{mode: mode, approve: approve}

	if mode == pb.ImportMode_IMPORT_MODE_REPLACE_ALL {
		if err := middleware.RequirePermission(ctx, s.db, permissions.DeleteEntry); err != nil {
			return options, err
		}
	}

	if approve {
		if err := middleware.RequirePermission(ctx, s.db, permissions.ReviewEntry); err != nil {
			return options, err
		}
	}

	if len(content) == 0 {
		return options, errors.InvalidInput(errors.ErrInvalidInput, "file_content is required")
	}

	if options.mode == pb.ImportMode_IMPORT_MODE_UNSPECIFIED {
		options.mode = pb.ImportMode_IMPORT_MODE_CREATE_OR_UPDATE
	}

	return options, nil
}

// importOptions say how an import treats existing entries and the review state
type importOptions struct {
	mode pb.ImportMode
	// Approves every imported translation; otherwise new and changed ones are drafts.
	approve bool
}

// newReview is the review state of a translation the import creates
func (o importOptions) newReview(userID *string) models.Review {
	review := models.Review{Status: models.StatusDraft}
	if o.approve {
		review.Approve(userID, time.Now())
	}

	return review
}

// requireLocaleID checks the locale a single-file import goes into
func requireLocaleID(localeID string) error {
	if localeID == "" {
		return errors.InvalidInput(errors.ErrInvalidInput, "locale_id is required")
	}

	return nil
}
