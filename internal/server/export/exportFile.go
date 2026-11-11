package export

import "context"

// exportFile renders the locale's file in the format, for the selected entries, and
// names it after the selected scopes.
func (s *Server) exportFile(
	ctx context.Context, format, localeCode string, scopeIDs, tagIDs []string, includeDrafts bool,
) (content, filename string, err error) {
	db := s.db.WithContext(ctx)

	locale, err := findLocale(db, localeCode)
	if err != nil {
		return "", "", err
	}

	sel, err := selection(db, scopeIDs, tagIDs, includeDrafts)
	if err != nil {
		return "", "", err
	}

	content, err = Render(db, format, locale, sel)
	if err != nil {
		return "", "", err
	}

	return content, exportFilename(db, sel.ScopeIDs, locale.Code, format), nil
}
