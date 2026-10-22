package localization

import (
	"context"
	"slices"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

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
	if err := middleware.RequirePermission(ctx, s.db, permissions.UpdateEntry); err != nil {
		return nil, err
	}

	// Translating loads the translation servers, so only some roles may ask for it.
	if err := middleware.RequirePermission(ctx, s.db, permissions.UseMachineTranslation); err != nil {
		return nil, err
	}

	if req.Msg.EntryId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "entry id is required")
	}

	var localeIDs []string
	for _, id := range req.Msg.TargetLocaleIds {
		if id != "" && !slices.Contains(localeIDs, id) {
			localeIDs = append(localeIDs, id)
		}
	}
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

	var entry models.Entry
	if err := s.db.First(&entry, "id = ?", req.Msg.EntryId).Error; err != nil {
		return nil, errors.NotFound(errors.ErrEntryNotFound, "entry not found")
	}

	var defaultLocale models.Locale
	if err := s.db.First(&defaultLocale, "is_default = ?", true).Error; err != nil {
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
