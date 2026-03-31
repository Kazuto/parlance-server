package converter

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/datatypes"
)

func TestLocaleToProto(t *testing.T) {
	now := time.Now()

	names := map[string]string{
		"en": "English",
		"de": "Englisch",
		"es": "Inglés",
	}
	namesJSON, _ := json.Marshal(names)

	tests := []struct {
		name          string
		locale        *models.Locale
		requestLocale string
		wantNil       bool
		expectedName  string
	}{
		{
			name:    "nil locale",
			locale:  nil,
			wantNil: true,
		},
		{
			name: "locale with English preference",
			locale: &models.Locale{
				ID:        "locale-id",
				Code:      "en",
				Names:     datatypes.JSON(namesJSON),
				IsDefault: true,
				CreatedAt: now,
				UpdatedAt: now,
			},
			requestLocale: "en",
			expectedName:  "English",
		},
		{
			name: "locale with German preference",
			locale: &models.Locale{
				ID:        "locale-id",
				Code:      "en",
				Names:     datatypes.JSON(namesJSON),
				IsDefault: false,
				CreatedAt: now,
				UpdatedAt: now,
			},
			requestLocale: "de",
			expectedName:  "Englisch",
		},
		{
			name: "locale with Spanish preference",
			locale: &models.Locale{
				ID:        "locale-id",
				Code:      "en",
				Names:     datatypes.JSON(namesJSON),
				IsDefault: false,
				CreatedAt: now,
				UpdatedAt: now,
			},
			requestLocale: "es",
			expectedName:  "Inglés",
		},
		{
			name: "locale with unsupported preference falls back to English",
			locale: &models.Locale{
				ID:        "locale-id",
				Code:      "en",
				Names:     datatypes.JSON(namesJSON),
				IsDefault: false,
				CreatedAt: now,
				UpdatedAt: now,
			},
			requestLocale: "fr",
			expectedName:  "English",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LocaleToProto(tt.locale, tt.requestLocale)

			if tt.wantNil {
				if result != nil {
					t.Errorf("LocaleToProto() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("LocaleToProto() returned nil")
			}

			if result.Code != tt.locale.Code {
				t.Errorf("Code = %v, want %v", result.Code, tt.locale.Code)
			}

			if result.Name != tt.expectedName {
				t.Errorf("Name = %v, want %v", result.Name, tt.expectedName)
			}

			if result.IsDefault != tt.locale.IsDefault {
				t.Errorf("IsDefault = %v, want %v", result.IsDefault, tt.locale.IsDefault)
			}

			// Check that all names are included
			if len(result.Names) != 3 {
				t.Errorf("Names length = %v, want 3", len(result.Names))
			}
		})
	}
}

func TestLocalesToProto(t *testing.T) {
	now := time.Now()

	names1 := map[string]string{"en": "English", "de": "Englisch"}
	names1JSON, _ := json.Marshal(names1)

	names2 := map[string]string{"en": "German", "de": "Deutsch"}
	names2JSON, _ := json.Marshal(names2)

	locales := []models.Locale{
		{
			ID:        "locale-1",
			Code:      "en",
			Names:     datatypes.JSON(names1JSON),
			IsDefault: true,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "locale-2",
			Code:      "de",
			Names:     datatypes.JSON(names2JSON),
			IsDefault: false,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	result := LocalesToProto(locales, "en")

	if len(result) != 2 {
		t.Fatalf("LocalesToProto() length = %v, want 2", len(result))
	}

	if result[0].Code != "en" {
		t.Errorf("First locale code = %v, want en", result[0].Code)
	}

	if result[0].Name != "English" {
		t.Errorf("First locale name = %v, want English", result[0].Name)
	}

	if result[1].Code != "de" {
		t.Errorf("Second locale code = %v, want de", result[1].Code)
	}
}
