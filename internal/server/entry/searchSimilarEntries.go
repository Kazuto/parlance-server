package entry

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/translate"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// The similar entries' limits, as the translation memory's: longer texts list none, as
// they are slow to compare, but still count their twins.
const (
	similarMaxLength    = 2000
	similarDefaultLimit = 5
	similarMaxLimit     = 20
	// How alike a default text must be, see translate.Similarity. Higher than the
	// translation memory's: a twin is a text that says the same, not one to learn from.
	similarMinSimilarity = 0.8
	// How many texts the trigram index picks for scoring
	similarCandidates = 50
)

// similarQuery picks the active entries whose default text shares enough trigrams with
// the searched one, the nearest first: the % operator uses the trigram index at
// pg_trgm's own threshold, and translate.Similarity then scores them.
const similarQuery = `
SELECT entries.id AS entry_id, entries.key, source.translation AS text
FROM localizations source
JOIN entries ON entries.id = source.entry_id AND entries.deleted_at IS NULL
WHERE source.locale_id = @source
	AND source.deleted_at IS NULL
	AND source.translation % @text
	AND source.entry_id::text <> @exclude
ORDER BY similarity(source.translation, @text) DESC, entries.key
LIMIT @candidates`

// sameTextQuery counts the active entries with the searched default text, compared as
// the duplicates filter compares them.
const sameTextQuery = `
SELECT COUNT(*)
FROM localizations source
JOIN entries ON entries.id = source.entry_id AND entries.deleted_at IS NULL
WHERE source.locale_id = @source
	AND source.deleted_at IS NULL
	AND LOWER(TRIM(source.translation)) = LOWER(TRIM(@text))
	AND source.entry_id::text <> @exclude`

// SearchSimilarEntries finds the entries whose default text is the same as or similar
// to the searched one
func (s *Server) SearchSimilarEntries(
	ctx context.Context,
	req *connect.Request[pb.SearchSimilarEntriesRequest],
) (*connect.Response[pb.SearchSimilarEntriesResponse], error) {
	response := &pb.SearchSimilarEntriesResponse{Entries: []*pb.SimilarEntry{}}

	if strings.TrimSpace(req.Msg.Text) == "" {
		return connect.NewResponse(response), nil
	}

	db := s.db.WithContext(ctx)

	defaultLocale, err := common.DefaultLocale(db)
	if err != nil {
		return nil, err
	}

	if defaultLocale == nil {
		return connect.NewResponse(response), nil
	}

	params := map[string]any{
		"text":       req.Msg.Text,
		"source":     defaultLocale.ID,
		"exclude":    req.Msg.ExcludeEntryId,
		"candidates": similarCandidates,
	}

	var sameText int64
	if err := db.Raw(sameTextQuery, params).Scan(&sameText).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to count entries with the same text")
	}

	response.SameTextCount = int32(sameText)

	if utf8.RuneCountInString(strings.TrimSpace(req.Msg.Text)) > similarMaxLength {
		return connect.NewResponse(response), nil
	}

	limit := req.Msg.Limit
	if limit <= 0 {
		limit = similarDefaultLimit
	}

	limit = min(limit, similarMaxLimit)

	var candidates []struct {
		EntryID string
		Key     string
		Text    string
	}

	if err := db.Raw(similarQuery, params).Scan(&candidates).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to search similar entries")
	}

	for _, candidate := range candidates {
		similarity := translate.Similarity(req.Msg.Text, candidate.Text)
		if similarity < similarMinSimilarity {
			continue
		}

		response.Entries = append(response.Entries, &pb.SimilarEntry{
			EntryId:    candidate.EntryID,
			Key:        candidate.Key,
			Text:       candidate.Text,
			Similarity: float32(similarity),
		})
	}

	slices.SortStableFunc(response.Entries, func(a, b *pb.SimilarEntry) int {
		return cmp.Or(cmp.Compare(b.Similarity, a.Similarity), cmp.Compare(a.Key, b.Key))
	})

	if len(response.Entries) > int(limit) {
		response.Entries = response.Entries[:limit]
	}

	return connect.NewResponse(response), nil
}
