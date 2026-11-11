package entry

import (
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// preloadDetails loads what an entry shows along with it: texts, scopes, active tags and
// pending releases
func preloadDetails(query *gorm.DB) *gorm.DB {
	return common.PreloadReleases(common.PreloadTags(query.Preload("Localizations").Preload("Scopes")))
}

// replaceReleases sets the entry's pending releases to releases, keeping the links it
// already has. Links to releases that went out stay, as their record.
func replaceReleases(tx *gorm.DB, entryID string, releases []models.Release, userID *string) error {
	releaseIDs := make([]string, len(releases))
	for i, release := range releases {
		releaseIDs[i] = release.ID
	}

	stale := tx.Where("entry_id = ? AND release_id IN (SELECT id FROM releases WHERE released_at IS NULL)", entryID)
	if len(releaseIDs) > 0 {
		stale = stale.Where("release_id NOT IN ?", releaseIDs)
	}

	if err := stale.Delete(&models.ReleaseEntry{}).Error; err != nil {
		return err
	}

	if len(releases) == 0 {
		return nil
	}

	links := make([]models.ReleaseEntry, len(releases))
	for i, release := range releases {
		links[i] = models.ReleaseEntry{ReleaseID: release.ID, EntryID: entryID, CreatedBy: userID}
	}

	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&links).Error
}
