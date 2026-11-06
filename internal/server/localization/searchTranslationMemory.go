package localization

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/translate"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// The translation memory's limits: texts outside these lengths find nothing, as short
// ones are alike by chance and long ones are slow to compare.
const (
	memoryMinLength    = 3
	memoryMaxLength    = 2000
	memoryDefaultLimit = 5
	memoryMaxLimit     = 20
	// How alike a default text must be, see translate.Similarity
	memoryMinSimilarity = 0.6
	// How many texts the trigram index picks for scoring
	memoryCandidates = 50
)

// memoryQuery picks the approved, current translations into the target locale of
// entries whose default text shares enough trigrams with the searched one: the %
// operator uses the trigram index at pg_trgm's own threshold. Trigrams are harsh on
// short texts and blind to word order, so they only pick candidates, which
// translate.Similarity then scores. DISTINCT ON lists each pair of texts once, under
// its first key.
const memoryQuery = `
SELECT * FROM (
	SELECT DISTINCT ON (source.translation, target.translation)
		entries.id AS entry_id,
		entries.key,
		source.translation AS source_text,
		target.translation,
		similarity(source.translation, @text) AS trigrams
	FROM localizations source
	JOIN entries ON entries.id = source.entry_id AND entries.deleted_at IS NULL
	JOIN localizations target ON target.entry_id = source.entry_id
		AND target.locale_id = @target
		AND target.deleted_at IS NULL
		AND target.status = @approved
		AND target.outdated_at IS NULL
	WHERE source.locale_id = @source
		AND source.deleted_at IS NULL
		AND source.translation % @text
		AND source.entry_id::text <> @exclude
	ORDER BY source.translation, target.translation, entries.key
) candidates
ORDER BY trigrams DESC, key
LIMIT @candidates`

// SearchTranslationMemory finds how similar default texts were translated before
func (s *Server) SearchTranslationMemory(
	ctx context.Context,
	req *connect.Request[pb.SearchTranslationMemoryRequest],
) (*connect.Response[pb.SearchTranslationMemoryResponse], error) {
	if req.Msg.TargetLocaleId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "target locale id is required")
	}

	db := s.db.WithContext(ctx)

	if _, err := common.FindByID[models.Locale](db, req.Msg.TargetLocaleId, errors.ErrLocaleNotFound, "target locale not found"); err != nil {
		return nil, err
	}

	defaultLocale, err := common.DefaultLocale(db)
	if err != nil {
		return nil, err
	}

	if defaultLocale != nil && defaultLocale.ID == req.Msg.TargetLocaleId {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "the target locale is the default locale")
	}

	response := &pb.SearchTranslationMemoryResponse{Matches: []*pb.TranslationMemoryMatch{}}

	length := utf8.RuneCountInString(strings.TrimSpace(req.Msg.SourceText))
	if defaultLocale == nil || length < memoryMinLength || length > memoryMaxLength {
		return connect.NewResponse(response), nil
	}

	limit := req.Msg.Limit
	if limit <= 0 {
		limit = memoryDefaultLimit
	}

	limit = min(limit, memoryMaxLimit)

	var candidates []struct {
		EntryID     string
		Key         string
		SourceText  string
		Translation string
	}

	err = db.Raw(memoryQuery, map[string]any{
		"text":       req.Msg.SourceText,
		"source":     defaultLocale.ID,
		"target":     req.Msg.TargetLocaleId,
		"approved":   models.StatusApproved,
		"exclude":    req.Msg.ExcludeEntryId,
		"candidates": memoryCandidates,
	}).Scan(&candidates).Error
	if err != nil {
		return nil, errors.InternalCause(err, "failed to search the translation memory")
	}

	for _, candidate := range candidates {
		similarity := translate.Similarity(req.Msg.SourceText, candidate.SourceText)
		if similarity < memoryMinSimilarity {
			continue
		}

		response.Matches = append(response.Matches, &pb.TranslationMemoryMatch{
			EntryId:     candidate.EntryID,
			Key:         candidate.Key,
			SourceText:  candidate.SourceText,
			Translation: candidate.Translation,
			Similarity:  float32(similarity),
		})
	}

	slices.SortStableFunc(response.Matches, func(a, b *pb.TranslationMemoryMatch) int {
		return cmp.Or(cmp.Compare(b.Similarity, a.Similarity), cmp.Compare(a.Key, b.Key))
	})

	if len(response.Matches) > int(limit) {
		response.Matches = response.Matches[:limit]
	}

	return connect.NewResponse(response), nil
}
