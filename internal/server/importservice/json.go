package importservice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"
)

// parseJSONTranslations accepts flat dotted keys and nested objects. Token-based
// decoding preserves duplicate keys so neither duplicates nor flattened-path
// collisions can silently overwrite a translation.
func parseJSONTranslations(content []byte) (map[string]string, error) {
	if !utf8.Valid(content) {
		return nil, fmt.Errorf("translation files must use UTF-8")
	}

	decoder := json.NewDecoder(bytes.NewReader(content))

	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	if token != json.Delim('{') {
		return nil, fmt.Errorf("expected a JSON object of translations")
	}

	translations := map[string]string{}

	var readObject func(string, int) error

	readObject = func(prefix string, depth int) error {
		if depth > 100 {
			return fmt.Errorf("JSON nesting exceeds 100 levels")
		}

		keys := map[string]bool{}

		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}

			key, ok := token.(string)
			if !ok || key == "" {
				return fmt.Errorf("empty or invalid key under %q", prefix)
			}

			path := key
			if prefix != "" {
				path = prefix + "." + key
			}

			if keys[key] {
				return fmt.Errorf("duplicate JSON key %q", path)
			}

			keys[key] = true

			token, err = decoder.Token()
			if err != nil {
				return err
			}

			switch value := token.(type) {
			case string:
				if _, exists := translations[path]; exists {
					return fmt.Errorf("flattened key collision at %q", path)
				}

				translations[path] = value
			case json.Delim:
				if value != '{' {
					return fmt.Errorf("translation %q must be a string or object; arrays are not supported", path)
				}

				if err := readObject(path, depth+1); err != nil {
					return err
				}
			default:
				return fmt.Errorf("translation %q must be a string or object", path)
			}
		}

		token, err := decoder.Token()
		if err != nil {
			return err
		}

		if token != json.Delim('}') {
			return fmt.Errorf("expected closing object at %q", prefix)
		}

		return nil
	}
	if err := readObject("", 1); err != nil {
		return nil, err
	}

	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected content after translation object")
	}

	return translations, nil
}
