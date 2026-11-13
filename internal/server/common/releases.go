package common

import (
	"time"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

// heldBackBy matches the release links, as re, of releases that hold their entries at
// a time: releases that haven't gone out and whose date, if they have one, hasn't come.
const heldBackBy = "SELECT 1 FROM release_entries re JOIN releases r ON r.id = re.release_id" +
	" WHERE re.entry_id = entries.id AND r.released_at IS NULL AND (r.release_at IS NULL OR r.release_at > ?)"

// ExcludeHeldBack drops the entries a release holds back from exports at now. An entry
// in several releases waits for all of them.
func ExcludeHeldBack(query *gorm.DB, now time.Time) *gorm.DB {
	return query.Where("NOT EXISTS ("+heldBackBy+")", now.UTC())
}

// FilterEntriesByReleases keeps entries in any of the releases; without releases it keeps all.
func FilterEntriesByReleases(query *gorm.DB, releaseIDs []string) *gorm.DB {
	if len(releaseIDs) == 0 {
		return query
	}

	return query.Where("entries.id IN (SELECT entry_id FROM release_entries WHERE release_id IN ?)", releaseIDs)
}

// PreloadReleases loads the entries' pending releases along with them, by date with
// unscheduled ones last
func PreloadReleases(query *gorm.DB) *gorm.DB {
	return query.Preload("Releases", func(tx *gorm.DB) *gorm.DB {
		return tx.Where("releases.released_at IS NULL").
			Order("releases.release_at IS NULL, releases.release_at, releases.name")
	})
}

// ResolvePendingReleases loads the releases with the IDs, ignoring blanks and duplicates.
// An unknown ID is invalid input; a release that went out can't take entries.
func ResolvePendingReleases(db *gorm.DB, ids []string) ([]models.Release, error) {
	releases, err := ResolveByIDs[models.Release](db.Model(&models.Release{}), ids, errors.ErrReleaseNotFound, "one or more releases do not exist")
	if err != nil {
		return nil, err
	}

	for _, release := range releases {
		if !release.Pending() {
			return nil, errors.FailedPrecondition(errors.ErrReleaseAlreadyReleased, "release "+release.Name+" went out already")
		}
	}

	return releases, nil
}
