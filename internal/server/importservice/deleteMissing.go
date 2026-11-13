package importservice

import (
	"archive/zip"
	"slices"
	"strings"
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
)

// noHeldKeys rejects a file without keys, which would delete every entry
const noHeldKeys = "delete_missing needs a file that holds entries"

// heldKeys returns the keys a file holds: those it imports and those it holds without
// importing them
func heldKeys(translations map[string]string, more []string) map[string]bool {
	held := make(map[string]bool, len(translations)+len(more))

	for key := range translations {
		held[key] = true
	}

	for _, key := range more {
		held[key] = true
	}

	return held
}

// deleteMissingEntries deletes the entries whose keys the file doesn't hold: those of the
// scopes, or all without scopes. An entry other scopes keep only leaves the import's,
// with its tags of them. A dry run lists the first of those entries by key, with their
// default texts.
func deleteMissingEntries(tx *gorm.DB, userID string, scopeIDs []string, held map[string]bool, dryRun bool) (*pb.ImportDeletion, error) {
	if len(held) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, noHeldKeys)
	}

	query := tx.Model(&models.Entry{}).Select("id", "key").Order("key")
	if len(scopeIDs) > 0 {
		query = query.Where("id IN (?)", tx.Table("entry_scopes").Select("entry_id").Where("scope_id IN ?", scopeIDs))
	}

	var entries []models.Entry
	if err := query.Find(&entries).Error; err != nil {
		return nil, err
	}

	entries = slices.DeleteFunc(entries, func(entry models.Entry) bool { return held[entry.Key] })
	deletion := &pb.ImportDeletion{}

	if len(entries) == 0 {
		return deletion, nil
	}

	ids := make([]string, len(entries))
	for i, entry := range entries {
		ids[i] = entry.ID
	}

	var shared []string
	if len(scopeIDs) > 0 {
		if err := tx.Model(&models.EntryScope{}).Distinct("entry_id").
			Where("entry_id IN ? AND scope_id NOT IN ?", ids, scopeIDs).
			Pluck("entry_id", &shared).Error; err != nil {
			return nil, err
		}
	}

	kept := make(map[string]bool, len(shared))
	for _, id := range shared {
		kept[id] = true
	}

	var deleteIDs []string

	for _, id := range ids {
		if !kept[id] {
			deleteIDs = append(deleteIDs, id)
		}
	}

	if dryRun {
		if err := listDeletion(tx, deletion, entries, kept); err != nil {
			return nil, err
		}
	}

	if len(shared) > 0 {
		// The entries' tags of the scopes go with them, as when scopes are removed in the app
		if err := tx.Where("entry_id IN ? AND scope_id IN ?", shared, scopeIDs).Delete(&models.EntryTag{}).Error; err != nil {
			return nil, err
		}

		if err := tx.Where("entry_id IN ? AND scope_id IN ?", shared, scopeIDs).Delete(&models.EntryScope{}).Error; err != nil {
			return nil, err
		}

		if err := tx.Model(&models.Entry{}).Where("id IN ?", shared).
			Updates(map[string]any{"updated_by": userID, "updated_at": time.Now()}).Error; err != nil {
			return nil, err
		}
	}

	if len(deleteIDs) > 0 {
		if err := tx.Model(&models.Entry{}).Where("id IN ?", deleteIDs).
			Updates(map[string]any{"updated_by": userID, "deleted_at": time.Now()}).Error; err != nil {
			return nil, err
		}
	}

	deletion.EntriesDeleted = int32(len(deleteIDs))
	deletion.EntriesUnscoped = int32(len(shared))

	return deletion, nil
}

// listDeletion lists the first entries a dry run deletes or takes out of the scopes,
// with their default texts
func listDeletion(tx *gorm.DB, deletion *pb.ImportDeletion, entries []models.Entry, kept map[string]bool) error {
	entries = entries[:min(len(entries), maxImportChanges)]

	ids := make([]string, len(entries))
	for i, entry := range entries {
		ids[i] = entry.ID
	}

	texts := map[string]string{}

	defaultLocale, err := common.DefaultLocale(tx)
	if err != nil {
		return err
	}

	if defaultLocale != nil {
		var sources []models.Localization
		if err := tx.Select("entry_id", "translation").
			Where("locale_id = ? AND entry_id IN ?", defaultLocale.ID, ids).
			Find(&sources).Error; err != nil {
			return err
		}

		for _, source := range sources {
			texts[source.EntryID] = source.Translation
		}
	}

	deletion.Entries = make([]*pb.ImportDeletedEntry, len(entries))
	for i, entry := range entries {
		deletion.Entries[i] = &pb.ImportDeletedEntry{
			Key: entry.Key, SourceTranslation: texts[entry.ID], Unscoped: kept[entry.ID],
		}
	}

	return nil
}

// zipHeldKeys returns the keys of the archive's files of the default language, which
// delete_missing keeps. Scope folders may hold several of them.
func zipHeldKeys(files []*zip.File, defaultLocale *models.Locale) (map[string]bool, error) {
	if defaultLocale == nil {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "delete_missing needs a default language")
	}

	held := map[string]bool{}
	found := false

	for _, file := range files {
		if !isZipLanguageFile(file) || !strings.EqualFold(zipLocaleCode(file), defaultLocale.Code) {
			continue
		}

		translations, err := readZipJSON(file)
		if err != nil {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "delete_missing can't read "+file.Name+": "+err.Error())
		}

		found = true

		for key := range translations {
			held[key] = true
		}
	}

	if !found {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "delete_missing needs the default language's file, "+defaultLocale.Code+".json")
	}

	// Checked before any file is imported
	if len(held) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, noHeldKeys)
	}

	return held, nil
}
