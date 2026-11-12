package release

import (
	"strings"
	"time"
	"unicode/utf8"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
)

const maxReleaseNameLength = 100

// releaseName trims a release's name and checks that it is set and fits the column
func releaseName(name string) (string, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "release name is required")
	}

	if utf8.RuneCountInString(name) > maxReleaseNameLength {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "release name is too long")
	}

	return name, nil
}

// checkReleaseName rejects a name another release has, ignoring case. exceptID is the
// release being renamed, if any.
func checkReleaseName(db *gorm.DB, name, exceptID string) error {
	query := db.Model(&models.Release{}).Where("LOWER(name) = LOWER(?)", name)
	if exceptID != "" {
		query = query.Where("id <> ?", exceptID)
	}

	taken, err := common.Exists(query)
	if err != nil {
		return errors.InternalCause(err, "failed to check release name")
	}

	if taken {
		return errors.AlreadyExists(errors.ErrReleaseAlreadyExists, "a release with this name already exists")
	}

	return nil
}

// releaseAt parses a release date, in RFC 3339, as UTC
func releaseAt(value string) (*time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "release date must be an RFC 3339 time")
	}

	parsed = parsed.UTC()

	return &parsed, nil
}

// findRelease loads the release with the ID
func findRelease(db *gorm.DB, id string) (models.Release, error) {
	if id == "" {
		return models.Release{}, errors.InvalidInput(errors.ErrInvalidInput, "release id is required")
	}

	return common.FindByID[models.Release](db, id, errors.ErrReleaseNotFound, "release not found")
}

// releasesToProto converts the releases with their counts of entries that aren't deleted
func releasesToProto(db *gorm.DB, releases []models.Release) ([]*pb.Release, error) {
	result := converter.ReleasesToProto(releases)
	if len(releases) == 0 {
		return result, nil
	}

	ids := make([]string, len(releases))
	for i, release := range releases {
		ids[i] = release.ID
	}

	var rows []struct {
		ReleaseID string
		Count     int32
	}

	if err := db.Model(&models.ReleaseEntry{}).
		Select("release_entries.release_id, COUNT(*) AS count").
		Joins("JOIN entries ON entries.id = release_entries.entry_id AND entries.deleted_at IS NULL").
		Where("release_entries.release_id IN ?", ids).
		Group("release_entries.release_id").
		Scan(&rows).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to count release entries")
	}

	counts := make(map[string]int32, len(rows))
	for _, row := range rows {
		counts[row.ReleaseID] = row.Count
	}

	for _, release := range result {
		release.EntryCount = counts[release.Id]
	}

	return result, nil
}

// releaseToProto converts the release with its count of entries
func releaseToProto(db *gorm.DB, release models.Release) (*pb.Release, error) {
	result, err := releasesToProto(db, []models.Release{release})
	if err != nil {
		return nil, err
	}

	return result[0], nil
}
