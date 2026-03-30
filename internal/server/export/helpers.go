package export

import "strings"

// setNestedKey sets a value in a nested map using dot notation
// e.g., "user.profile.name" -> {"user": {"profile": {"name": value}}}
func setNestedKey(m map[string]interface{}, key string, value string) {
	parts := strings.Split(key, ".")

	// If no dot notation, just set directly
	if len(parts) == 1 {
		m[key] = value
		return
	}

	// Navigate/create nested maps
	current := m
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]

		if _, exists := current[part]; !exists {
			current[part] = make(map[string]interface{})
		}

		// Type assertion to continue navigating
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			// If the key already exists as a non-map value, override it
			current[part] = make(map[string]interface{})
			current = current[part].(map[string]interface{})
		}
	}

	// Set the final value
	current[parts[len(parts)-1]] = value
}
