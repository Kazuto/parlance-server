package export

import (
	"testing"
)

func TestSetNestedKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected map[string]interface{}
	}{
		{
			name:  "simple key",
			key:   "hello",
			value: "world",
			expected: map[string]interface{}{
				"hello": "world",
			},
		},
		{
			name:  "two level nested key",
			key:   "user.name",
			value: "John",
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "John",
				},
			},
		},
		{
			name:  "three level nested key",
			key:   "user.profile.email",
			value: "john@example.com",
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"profile": map[string]interface{}{
						"email": "john@example.com",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(map[string]interface{})
			setNestedKey(result, tt.key, tt.value)

			if !mapsEqual(result, tt.expected) {
				t.Errorf("setNestedKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSetNestedKeyMultipleKeys(t *testing.T) {
	m := make(map[string]interface{})

	setNestedKey(m, "user.profile.name", "John")
	setNestedKey(m, "user.profile.email", "john@example.com")
	setNestedKey(m, "user.settings.theme", "dark")

	expected := map[string]interface{}{
		"user": map[string]interface{}{
			"profile": map[string]interface{}{
				"name":  "John",
				"email": "john@example.com",
			},
			"settings": map[string]interface{}{
				"theme": "dark",
			},
		},
	}

	if !mapsEqual(m, expected) {
		t.Errorf("Multiple setNestedKey() calls = %v, want %v", m, expected)
	}
}

// Helper function to compare nested maps
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}

		switch vt := v.(type) {
		case map[string]interface{}:
			bvt, ok := bv.(map[string]interface{})
			if !ok {
				return false
			}
			if !mapsEqual(vt, bvt) {
				return false
			}
		default:
			if v != bv {
				return false
			}
		}
	}

	return true
}
