package localization

import (
	"context"
	"slices"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// maxSuggestionLocales limits how many languages one request may ask for.
const maxSuggestionLocales = 50

// SuggestTranslations translates an entry's default-locale text into the given
// locales. Nothing is saved: the editor shows the suggestions as drafts to review.
func (s *Server) SuggestTranslations(
	ctx context.Context,
	req *connect.Request[pb.SuggestTranslationsRequest],
) (*connect.Response[pb.SuggestTranslationsResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	localeIDs := common.UniqueIDs(req.Msg.TargetLocaleIds)
	if len(localeIDs) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "target locale ids are required")
	}

	if len(localeIDs) > maxSuggestionLocales {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "too many target locales")
	}

	translator, err := s.translator(ctx)
	if err != nil {
		return nil, err
	}

	entry, err := common.FindByID[models.Entry](s.db.WithContext(ctx), req.Msg.EntryId, errors.ErrEntryNotFound, "entry not found")
	if err != nil {
		return nil, err
	}

	defaultLocale, err := common.DefaultLocale(s.db.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	if defaultLocale == nil {
		return nil, errors.FailedPrecondition(errors.ErrFailedPrecondition, "no default locale is set")
	}

	var source models.Localization

	err = s.db.First(&source, "entry_id = ? AND locale_id = ?", entry.ID, defaultLocale.ID).Error
	if err != nil || source.Translation == "" {
		return nil, errors.FailedPrecondition(errors.ErrFailedPrecondition, "the entry has no text in the default locale")
	}

	var found []models.Locale
	if err := s.db.Where("id IN ?", localeIDs).Find(&found).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to load locales")
	}

	locales := make([]models.Locale, 0, len(localeIDs))

	for _, id := range localeIDs {
		index := slices.IndexFunc(found, func(locale models.Locale) bool { return locale.ID == id })
		if index < 0 {
			return nil, errors.NotFound(errors.ErrLocaleNotFound, "target locale not found")
		}

		locales = append(locales, found[index])
	}

	terms, err := s.glossaryTerms(source.Translation, localeIDs)
	if err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to load the glossary")
	}

	return connect.NewResponse(&pb.SuggestTranslationsResponse{
		Suggestions: s.suggest(ctx, translator, source.Translation, locales, terms),
	}), nil
}
