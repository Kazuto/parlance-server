package export

import (
	"context"

	"github.com/kazuto/parlance-server/internal/webhook"
)

// exportFile renders the locale's file in the format, for the selected entries, and
// names it after the selected scopes.
func (s *Server) exportFile(
	ctx context.Context, format, localeCode string, req selectionRequest,
) (content, filename string, err error) {
	defer func() {
		if err != nil {
			s.reportFailure(ctx, err, webhook.ExportFailedData{
				Format:   format,
				Locales:  []string{localeCode},
				ScopeIDs: req.GetScopeIds(),
				TagIDs:   req.GetTagIds(),
			})
		}
	}()

	db := s.db.WithContext(ctx)

	locale, err := findLocale(db, localeCode)
	if err != nil {
		return "", "", err
	}

	sel, err := selection(db, req)
	if err != nil {
		return "", "", err
	}

	content, err = Render(db, format, locale, sel)
	if err != nil {
		return "", "", err
	}

	return content, exportFilename(db, sel.ScopeIDs, locale.Code, format), nil
}
