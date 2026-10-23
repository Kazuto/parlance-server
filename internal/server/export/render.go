package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/gettext"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

// Formats a locale's translations can be rendered in, named by their file extension
const (
	FormatJSON = "json"
	FormatPHP  = "php"
	FormatPO   = "po"
)

// Formats lists the formats Render takes.
var Formats = []string{FormatJSON, FormatPHP, FormatPO}

// Render renders the locale's translations of the entries in any of the scopes (all
// entries without scopes) as a file in the format. Drafts are left out unless included.
func Render(db *gorm.DB, format string, locale models.Locale, scopeIDs []string, includeDrafts bool) (string, error) {
	entries, err := loadEntries(db, locale.ID, scopeIDs, includeDrafts)
	if err != nil {
		return "", err
	}

	switch format {
	case FormatJSON:
		return renderJSON(entries)
	case FormatPHP:
		return renderPHP(entries), nil
	case FormatPO:
		return renderPO(locale.Code, entries), nil
	default:
		return "", errors.InvalidInput(errors.ErrInvalidInput, "unknown export format "+format)
	}
}

// renderJSON renders a flat object of keys and translations
func renderJSON(entries []exportedEntry) (string, error) {
	content, err := marshalJSON(translationMap(entries))
	if err != nil {
		return "", errors.InternalCause(err, "failed to marshal JSON")
	}

	return string(content), nil
}

// marshalJSON encodes an exported file as people write them, so exports into a
// repository only change what changed: indented by two spaces, with "&", "<" and
// ">" as they are, and ending with one newline.
func marshalJSON(value any) ([]byte, error) {
	var content bytes.Buffer

	encoder := json.NewEncoder(&content)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	// Encode ends the file with a newline.
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	return content.Bytes(), nil
}

// renderPHP renders a Laravel language file: a PHP array of keys and translations
func renderPHP(entries []exportedEntry) string {
	var content strings.Builder

	content.WriteString("<?php\n\nreturn [\n")

	for _, entry := range entries {
		if entry.Translated {
			fmt.Fprintf(&content, "    %s => %s,\n", phpString(entry.Key), phpString(entry.Translation))
		}
	}

	content.WriteString("];\n")

	return content.String()
}

// renderPO renders a gettext catalog with every entry, untranslated ones empty
func renderPO(localeCode string, entries []exportedEntry) string {
	messages := make([]gettext.Entry, len(entries))
	for i, entry := range entries {
		messages[i] = gettext.Entry{ID: entry.Key, Description: entry.Description, Translation: entry.Translation}
	}

	return gettext.Marshal(localeCode, messages)
}
