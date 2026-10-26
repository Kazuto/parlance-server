package importservice

import (
	"cmp"
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
	// changes lists the translations whose text the import adds, changes or removes
	changes []importChange
}

// importChange is a translation whose text an import adds, changes or removes
type importChange struct {
	// The entry's id, unless the import creates it
	entryID string
	change  *pb.ImportChange
}

// importTarget is the locale an import writes to
type importTarget struct {
	localeID string
	// Whether it is the default locale, whose changes outdate the other translations
	source bool
	// The translations REPLACE_ALL deleted before importing, by entry id
	replaced map[string]models.Localization
}

// maxImportChanges is how many changes a dry run lists
const maxImportChanges = 500

// errDryRun rolls back the transaction of a dry run
var errDryRun = stderrors.New("dry run")

// runImport runs an import's writes. A dry run makes them in a transaction that it then
// rolls back, so its result tells what the import would change. The files of a ZIP share
// that transaction, so each sees the ones before, as when the import is kept.
func (s *Server) runImport(ctx context.Context, dryRun bool, run func(db *gorm.DB) error) error {
	db := s.db.WithContext(ctx)
	if !dryRun {
		return run(db)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := run(tx); err != nil {
			return err
		}

		return errDryRun
	})

	switch {
	case stderrors.Is(err, errDryRun):
		return nil
	case err == nil || connect.CodeOf(err) != connect.CodeUnknown:
		return err
	}

	log.Printf("import dry run failed: %v", err)

	return errors.Internal(errors.ErrInternal, "import failed")
}

// processImport imports the translations of a single file
func (s *Server) processImport(
	ctx context.Context,
	localeID string,
	scopeIDs []string,
	translations map[string]string,
	options importOptions,
) (*pb.ImportResult, error) {
	var result *pb.ImportResult

	err := s.runImport(ctx, options.dryRun, func(db *gorm.DB) (err error) {
		result, err = importTranslations(ctx, db, localeID, scopeIDs, translations, options)

		return err
	})

	return result, err
}

// importTranslations handles the common import logic for all formats. All reads and
// writes, including the REPLACE_ALL deletion, run in one transaction, so a failed import
// changes nothing. A dry run sends nothing to webhooks or tasks.
func importTranslations(
	ctx context.Context,
	db *gorm.DB,
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

	err := db.Transaction(func(tx *gorm.DB) error {
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

		outdated, err := common.MarkOutdated(tx, plan.outdatingEntryIDs, localeID, at)
		if err != nil {
			return err
		}

		result.TranslationsOutdated = int32(outdated)

		if err := summarizeChanges(tx, result, plan, target, options.dryRun); err != nil {
			return err
		}

		if options.dryRun {
			return nil
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
	// The existing entries the import writes to, by id
	imported := map[string]bool{}
	add := func(entryID string, change *pb.ImportChange) {
		plan.changes = append(plan.changes, importChange{entryID: entryID, change: change})
	}

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
			add("", &pb.ImportChange{Key: key, Action: pb.ImportChangeAction_IMPORT_CHANGE_ACTION_ADD, NewTranslation: value})

			continue
		}

		imported[entry.ID] = true

		if hasScopes {
			plan.entriesToAddScopes = append(plan.entriesToAddScopes, entry)
		}

		if localization, ok := localizationMap[entry.ID]; ok {
			// Unchanged text keeps its review state, unless the import approves it. Changed
			// or approved text is written for the default text as it is now.
			if localization.Translation != value {
				add(entry.ID, &pb.ImportChange{
					Key: key, Action: pb.ImportChangeAction_IMPORT_CHANGE_ACTION_CHANGE,
					OldTranslation: localization.Translation, NewTranslation: value,
				})

				localization.Translation = value
				localization.MarkDraft()
				localization.WrittenBy("")
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

			switch {
			case !replaced:
				add(entry.ID, &pb.ImportChange{Key: key, Action: pb.ImportChangeAction_IMPORT_CHANGE_ACTION_ADD, NewTranslation: value})
			case !unchanged:
				add(entry.ID, &pb.ImportChange{
					Key: key, Action: pb.ImportChangeAction_IMPORT_CHANGE_ACTION_CHANGE,
					OldTranslation: previous.Translation, NewTranslation: value,
				})
			}

			plan.localizationsToCreate = append(plan.localizationsToCreate, localization)
			result.EntriesCreated++
		}
	}

	if err := planRemovals(tx, plan, target, imported); err != nil {
		return nil, err
	}

	return plan, nil
}

// planRemovals lists the translations REPLACE_ALL deleted and the import doesn't bring
// back. Those of deleted entries aren't listed.
func planRemovals(tx *gorm.DB, plan *importPlan, target importTarget, imported map[string]bool) error {
	var entryIDs []string

	for entryID := range target.replaced {
		if !imported[entryID] {
			entryIDs = append(entryIDs, entryID)
		}
	}

	if len(entryIDs) == 0 {
		return nil
	}

	var entries []models.Entry
	if err := tx.Select("id", "key").Where("id IN ?", entryIDs).Find(&entries).Error; err != nil {
		return err
	}

	for _, entry := range entries {
		plan.changes = append(plan.changes, importChange{entryID: entry.ID, change: &pb.ImportChange{
			Key: entry.Key, Action: pb.ImportChangeAction_IMPORT_CHANGE_ACTION_REMOVE,
			OldTranslation: target.replaced[entry.ID].Translation,
		}})
	}

	return nil
}

// summarizeChanges counts the import's changes. A dry run also lists the first ones by
// key, with their entries' default texts when the import is in another locale.
func summarizeChanges(tx *gorm.DB, result *pb.ImportResult, plan *importPlan, target importTarget, dryRun bool) error {
	for _, change := range plan.changes {
		switch change.change.Action {
		case pb.ImportChangeAction_IMPORT_CHANGE_ACTION_ADD:
			result.TranslationsAdded++
		case pb.ImportChangeAction_IMPORT_CHANGE_ACTION_CHANGE:
			result.TranslationsChanged++
		case pb.ImportChangeAction_IMPORT_CHANGE_ACTION_REMOVE:
			result.TranslationsRemoved++
		}
	}

	if !dryRun || len(plan.changes) == 0 {
		return nil
	}

	changes := slices.SortedFunc(slices.Values(plan.changes), func(a, b importChange) int {
		return cmp.Compare(a.change.Key, b.change.Key)
	})
	changes = changes[:min(len(changes), maxImportChanges)]

	result.Changes = make([]*pb.ImportChange, len(changes))
	for i, change := range changes {
		result.Changes[i] = change.change
	}

	if target.source {
		return nil
	}

	defaultLocale, err := common.DefaultLocale(tx)
	if err != nil || defaultLocale == nil {
		return err
	}

	var entryIDs []string

	for _, change := range changes {
		if change.entryID != "" {
			entryIDs = append(entryIDs, change.entryID)
		}
	}

	if len(entryIDs) == 0 {
		return nil
	}

	var sources []models.Localization
	if err := tx.Select("entry_id", "translation").
		Where("locale_id = ? AND entry_id IN ?", defaultLocale.ID, entryIDs).
		Find(&sources).Error; err != nil {
		return err
	}

	texts := make(map[string]string, len(sources))
	for _, source := range sources {
		texts[source.EntryID] = source.Translation
	}

	for _, change := range changes {
		change.change.SourceTranslation = texts[change.entryID]
	}

	return nil
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
// A dry run needs the same.
func (s *Server) startImport(ctx context.Context, mode pb.ImportMode, approve, dryRun bool, content []byte) (importOptions, error) {
	options := importOptions{mode: mode, approve: approve, dryRun: dryRun}

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
	// Rolls the import back and lists its changes instead
	dryRun bool
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
