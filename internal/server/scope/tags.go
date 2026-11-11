package scope

import (
	"strings"
	"unicode/utf8"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
)

const maxTagNameLength = 100

// tagName trims a tag's name and checks that it is set and fits the column
func tagName(name string) (string, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "tag name is required")
	}

	if utf8.RuneCountInString(name) > maxTagNameLength {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "tag name is too long")
	}

	return name, nil
}

// checkTagName rejects a name another tag of the scope has, ignoring case. exceptID is
// the tag being renamed, if any.
func checkTagName(db *gorm.DB, scopeID, name, exceptID string) error {
	query := db.Model(&models.Tag{}).Where("scope_id = ? AND LOWER(name) = LOWER(?)", scopeID, name)
	if exceptID != "" {
		query = query.Where("id <> ?", exceptID)
	}

	taken, err := common.Exists(query)
	if err != nil {
		return errors.InternalCause(err, "failed to check tag name")
	}

	if taken {
		return errors.AlreadyExists(errors.ErrTagAlreadyExists, "a tag with this name already exists in the scope")
	}

	return nil
}

// fillTagCounts sets entry_count on the scopes' tags: their entries that aren't deleted
func fillTagCounts(db *gorm.DB, scopes []*pb.Scope) error {
	var tagIDs []string

	for _, scope := range scopes {
		for _, tag := range scope.Tags {
			tagIDs = append(tagIDs, tag.Id)
		}
	}

	if len(tagIDs) == 0 {
		return nil
	}

	var rows []struct {
		TagID string
		Count int32
	}

	if err := db.Model(&models.EntryTag{}).
		Select("entry_tags.tag_id, COUNT(*) AS count").
		Joins("JOIN entries ON entries.id = entry_tags.entry_id AND entries.deleted_at IS NULL").
		Where("entry_tags.tag_id IN ?", tagIDs).
		Group("entry_tags.tag_id").
		Scan(&rows).Error; err != nil {
		return errors.InternalCause(err, "failed to count tagged entries")
	}

	counts := make(map[string]int32, len(rows))
	for _, row := range rows {
		counts[row.TagID] = row.Count
	}

	for _, scope := range scopes {
		for _, tag := range scope.Tags {
			tag.EntryCount = counts[tag.Id]
		}
	}

	return nil
}

// preloadTags loads the scopes' tags by name along with them
func preloadTags(query *gorm.DB) *gorm.DB {
	return query.Preload("Tags", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("tags.name")
	})
}
