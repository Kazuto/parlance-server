package entry

import (
	"strings"

	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// The duplicates filter compares texts ignoring case and surrounding space, in SQL, so
// that it groups exactly what it lists.

// defaultTextOf is the listed entry's default text as the filter compares it, to list
// twins together. It finds the default locale itself, so it takes no parameters and
// can be an ORDER BY column: GORM drops an ORDER BY expression with parameters when
// the filter's sort is added after it.
const defaultTextOf = "(SELECT LOWER(TRIM(dt.translation)) FROM localizations dt" +
	" JOIN locales dl ON dl.id = dt.locale_id AND dl.is_default = TRUE AND dl.deleted_at IS NULL" +
	" WHERE dt.entry_id = entries.id AND dt.deleted_at IS NULL)"

// duplicateCursorSeparator joins the default text and the key in a cursor while the
// duplicates filter is set. Postgres text can't hold it, so neither part does.
const duplicateCursorSeparator = "\x00"

// duplicateSelection is what the scope, tag and release filters select: twins are looked
// for among these entries.
type duplicateSelection struct {
	scopeIDs   []string
	tagIDs     []string
	releaseIDs []string
}

// filterDuplicates keeps the entries that share their default text with another of the
// selected entries, or only those whose shared text has differing translations in one of
// the locales (all active ones when none are given). Blank texts and translations are
// never alike.
func filterDuplicates(query *gorm.DB, filter pb.DuplicateFilter, selection duplicateSelection, localeIDs []string) *gorm.DB {
	// Start separate statements so the subqueries don't inherit entry filters.
	db := query.Session(&gorm.Session{NewDB: true})

	// The selected active entries with their default texts, as the filter compares them;
	// a fresh statement for each use, as GORM changes a statement it chains on.
	texts := func() *gorm.DB {
		texts := db.Table("localizations source").
			Select("source.entry_id, LOWER(TRIM(source.translation)) AS text").
			Joins("JOIN locales ON locales.id = source.locale_id AND locales.is_default = TRUE AND locales.deleted_at IS NULL").
			Joins("JOIN entries ON entries.id = source.entry_id AND entries.deleted_at IS NULL").
			Where("source.deleted_at IS NULL AND TRIM(source.translation) <> ''")
		texts = common.FilterEntriesBySelection(texts, selection.scopeIDs, selection.tagIDs)

		return common.FilterEntriesByReleases(texts, selection.releaseIDs)
	}

	// The texts several entries share
	shared := db.Table("(?) AS shared", texts()).Select("shared.text").Group("shared.text")

	if filter == pb.DuplicateFilter_DUPLICATE_FILTER_TRANSLATED_DIFFERENTLY {
		// The shared texts with two different translations into one locale: an entry has
		// one translation per locale, so two come from two entries.
		shared = shared.
			Joins("JOIN localizations tr ON tr.entry_id = shared.entry_id AND tr.deleted_at IS NULL AND TRIM(tr.translation) <> ''").
			Joins("JOIN locales ON locales.id = tr.locale_id AND locales.deleted_at IS NULL").
			Group("tr.locale_id").
			Having("COUNT(DISTINCT LOWER(TRIM(tr.translation))) > 1")
		if len(localeIDs) > 0 {
			shared = shared.Where("tr.locale_id IN ?", localeIDs)
		}
	} else {
		shared = shared.Having("COUNT(*) > 1")
	}

	twins := db.Table("(?) AS twins", texts()).Select("twins.entry_id").Where("twins.text IN (?)", shared)

	return query.Where("entries.id IN (?)", twins)
}

// encodeDuplicateCursor is the cursor after an entry while the duplicates filter lists
// twins together: its default text, then its key.
func encodeDuplicateCursor(text, key string) string {
	return encodeCursor(text + duplicateCursorSeparator + key)
}

// decodeDuplicateCursor splits a cursor from encodeDuplicateCursor; ok is false for any
// other cursor.
func decodeDuplicateCursor(cursor string) (text, key string, ok bool) {
	value, err := decodeCursor(cursor)
	if err != nil {
		return "", "", false
	}

	return strings.Cut(value, duplicateCursorSeparator)
}
