// Package gettext reads and writes the singular, key-based PO format used by Parlance.
package gettext

import (
	"bufio"
	"bytes"
	"fmt"
	"mime"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Entry holds a PO record. Contextual, plural and fuzzy records are parsed so
// callers can report them rather than silently losing their meaning.
type Entry struct {
	ID          string
	Translation string
	Description string
	Context     bool
	Plural      bool
	Fuzzy       bool
	Obsolete    bool
}

type record struct {
	Entry
	idSet, translationSet bool
	contextSet, pluralSet bool
	last                  string
	indexed               map[int]bool
}

// Parse validates UTF-8 PO syntax, concatenates quoted continuation lines, and
// returns records in file order. The metadata record (empty msgid) is omitted.
func Parse(data []byte) ([]Entry, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("PO files must be UTF-8 encoded")
	}

	var entries []Entry

	var current record

	var headerSeen bool

	flush := func() error {
		if !current.idSet {
			if current.contextSet || current.pluralSet || current.translationSet {
				return fmt.Errorf("record is missing msgid")
			}

			current = record{}

			return nil
		}

		if !current.translationSet {
			return fmt.Errorf("record %q is missing msgstr", current.ID)
		}

		if current.ID == "" && !current.Context && !current.Plural && !current.Obsolete {
			if headerSeen {
				return fmt.Errorf("duplicate metadata record")
			}

			headerSeen = true

			for _, line := range strings.Split(current.Translation, "\n") {
				key, value, ok := strings.Cut(line, ":")
				if ok && strings.EqualFold(strings.TrimSpace(key), "Content-Type") {
					_, params, err := mime.ParseMediaType(strings.TrimSpace(value))
					if err != nil {
						return fmt.Errorf("invalid Content-Type header: %w", err)
					}

					if charset := params["charset"]; charset != "" && !strings.EqualFold(charset, "UTF-8") {
						return fmt.Errorf("unsupported charset %q; PO files must use UTF-8", charset)
					}
				}
			}
		} else {
			entries = append(entries, current.Entry)
		}

		current = record{}

		return nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), len(data)+1)

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if err := flush(); err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNumber, err)
			}

			continue
		}

		obsolete := strings.HasPrefix(line, "#~")
		if obsolete {
			line = strings.TrimSpace(strings.TrimPrefix(line, "#~"))
		}

		if strings.HasPrefix(line, "#") {
			if current.translationSet {
				if err := flush(); err != nil {
					return nil, fmt.Errorf("line %d: %w", lineNumber, err)
				}
			}

			switch {
			case strings.HasPrefix(line, "#,"):
				for _, flag := range strings.Split(line[2:], ",") {
					current.Fuzzy = current.Fuzzy || strings.TrimSpace(flag) == "fuzzy"
				}
			case strings.HasPrefix(line, "#."):
				// Extracted comments hold the description, one line each
				comment := strings.TrimPrefix(strings.TrimPrefix(line, "#."), " ")
				if current.Description != "" {
					comment = current.Description + "\n" + comment
				}

				current.Description = comment
			}

			continue
		}

		if strings.HasPrefix(line, "\"") {
			value, err := decode(line)
			if err != nil || current.last == "" {
				return nil, fmt.Errorf("line %d: invalid quoted continuation", lineNumber)
			}

			switch current.last {
			case "msgid":
				current.ID += value
			case "msgstr":
				current.Translation += value
			}

			continue
		}

		separator := strings.IndexAny(line, " \t")
		if separator < 0 {
			return nil, fmt.Errorf("line %d: expected a PO directive and quoted string", lineNumber)
		}

		keyword, quoted := line[:separator], line[separator+1:]
		if (keyword == "msgid" && current.idSet) || (keyword == "msgctxt" && current.translationSet) {
			if err := flush(); err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNumber, err)
			}
		}

		value, err := decode(strings.TrimSpace(quoted))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}

		current.Obsolete = current.Obsolete || obsolete

		switch keyword {
		case "msgctxt":
			if current.contextSet || current.idSet {
				return nil, fmt.Errorf("line %d: misplaced msgctxt", lineNumber)
			}

			current.Context, current.contextSet = true, true
		case "msgid":
			current.ID, current.idSet = value, true
		case "msgid_plural":
			if !current.idSet || current.pluralSet || current.translationSet {
				return nil, fmt.Errorf("line %d: misplaced msgid_plural", lineNumber)
			}

			current.Plural, current.pluralSet = true, true
		case "msgstr":
			if !current.idSet || current.translationSet || current.Plural {
				return nil, fmt.Errorf("line %d: misplaced msgstr", lineNumber)
			}

			current.Translation, current.translationSet = value, true
		default:
			if !strings.HasPrefix(keyword, "msgstr[") || !strings.HasSuffix(keyword, "]") || !current.pluralSet {
				return nil, fmt.Errorf("line %d: unsupported or misplaced directive %q", lineNumber, keyword)
			}

			index, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(keyword, "msgstr["), "]"))
			if err != nil || index < 0 || current.indexed[index] {
				return nil, fmt.Errorf("line %d: invalid or duplicate plural index", lineNumber)
			}

			if current.indexed == nil {
				current.indexed = map[int]bool{}
			}

			current.indexed[index] = true
			current.translationSet = true
		}

		current.last = keyword
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if err := flush(); err != nil {
		return nil, err
	}

	return entries, nil
}

func decode(quoted string) (string, error) {
	if len(quoted) < 2 || quoted[0] != '"' || quoted[len(quoted)-1] != '"' {
		return "", fmt.Errorf("expected a double-quoted PO string")
	}

	var value strings.Builder

	for i := 1; i < len(quoted)-1; i++ {
		c := quoted[i]
		if c == '"' || c < 32 {
			return "", fmt.Errorf("unescaped quote or control character")
		}

		if c != '\\' {
			value.WriteByte(c)

			continue
		}

		i++
		if i >= len(quoted)-1 {
			return "", fmt.Errorf("incomplete PO escape")
		}

		c = quoted[i]
		switch c {
		case '\\', '"', '\'':
			value.WriteByte(c)
		case 'a':
			value.WriteByte('\a')
		case 'b':
			value.WriteByte('\b')
		case 'f':
			value.WriteByte('\f')
		case 'n':
			value.WriteByte('\n')
		case 'r':
			value.WriteByte('\r')
		case 't':
			value.WriteByte('\t')
		case 'v':
			value.WriteByte('\v')
		default:
			start, base, limit := i, 8, 3

			if c == 'x' {
				i++
				start, base, limit = i, 16, len(quoted)
			}

			for i < len(quoted)-1 && i-start < limit {
				digit := quoted[i]
				valid := digit >= '0' && digit <= '7'

				if base == 16 {
					valid = digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f' || digit >= 'A' && digit <= 'F'
				}

				if !valid {
					break
				}

				i++
			}

			if start == i {
				return "", fmt.Errorf("invalid PO escape")
			}

			number, err := strconv.ParseUint(quoted[start:i], base, 8)
			if err != nil {
				return "", fmt.Errorf("invalid numeric PO escape")
			}

			value.WriteByte(byte(number))

			i--
		}
	}

	if !utf8.ValidString(value.String()) {
		return "", fmt.Errorf("invalid UTF-8 in PO string")
	}

	return value.String(), nil
}

func quote(value string) string {
	var output strings.Builder

	output.WriteByte('"')

	for _, c := range value {
		switch c {
		case '\\', '"':
			output.WriteByte('\\')
			output.WriteRune(c)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if c < 32 || c == 127 {
				fmt.Fprintf(&output, "\\%03o", c)
			} else {
				output.WriteRune(c)
			}
		}
	}

	output.WriteByte('"')

	return output.String()
}

// Marshal emits deterministic UTF-8 PO output, with entry keys as msgid and
// descriptions as extracted comments. Untranslated entries use empty msgstr.
func Marshal(language string, entries []Entry) string {
	var output strings.Builder

	output.WriteString("msgid \"\"\nmsgstr \"\"\n")

	for _, header := range []string{
		"Project-Id-Version: Parlance\n", "Language: " + strings.ReplaceAll(strings.ReplaceAll(language, "\n", ""), "\r", "") + "\n",
		"MIME-Version: 1.0\n", "Content-Type: text/plain; charset=UTF-8\n",
		"Content-Transfer-Encoding: 8bit\n", "X-Generator: Parlance\n",
	} {
		output.WriteString(quote(header) + "\n")
	}

	sorted := append([]Entry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	for _, entry := range sorted {
		output.WriteByte('\n')

		if entry.Description != "" {
			for _, line := range strings.Split(strings.ReplaceAll(entry.Description, "\r", ""), "\n") {
				output.WriteString("#. " + line + "\n")
			}
		}

		output.WriteString("msgid " + quote(entry.ID) + "\n")
		output.WriteString("msgstr " + quote(entry.Translation) + "\n")
	}

	return output.String()
}
