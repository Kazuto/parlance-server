package converter

import (
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
)

func TestEntryToProto(t *testing.T) {
	now := time.Now()
	userID := "user-123"

	tests := []struct {
		name          string
		entry         *models.Entry
		requestLocale string
		wantNil       bool
	}{
		{
			name:    "nil entry",
			entry:   nil,
			wantNil: true,
		},
		{
			name: "basic entry",
			entry: &models.Entry{
				ID:          "entry-1",
				Key:         "button.submit",
				Description: "Submit button",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			requestLocale: "en",
			wantNil:       false,
		},
		{
			name: "entry with user tracking",
			entry: &models.Entry{
				ID:          "entry-1",
				Key:         "button.submit",
				Description: "Submit button",
				CreatedBy:   &userID,
				UpdatedBy:   &userID,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			requestLocale: "en",
			wantNil:       false,
		},
		{
			name: "entry with localizations and scopes",
			entry: &models.Entry{
				ID:          "entry-1",
				Key:         "button.submit",
				Description: "Submit button",
				CreatedAt:   now,
				UpdatedAt:   now,
				Localizations: []models.Localization{
					{
						ID:          "loc-1",
						EntryID:     "entry-1",
						LocaleID:    "en",
						Translation: "Submit",
						CreatedAt:   now,
						UpdatedAt:   now,
					},
				},
				Scopes: []models.Scope{
					{
						ID:          "scope-1",
						Name:        "frontend",
						Description: "Frontend scope",
						CreatedAt:   now,
						UpdatedAt:   now,
					},
				},
			},
			requestLocale: "en",
			wantNil:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EntryToProto(tt.entry, tt.requestLocale)

			if tt.wantNil {
				if result != nil {
					t.Errorf("EntryToProto() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("EntryToProto() returned nil")
			}

			if result.Id != tt.entry.ID {
				t.Errorf("ID = %v, want %v", result.Id, tt.entry.ID)
			}

			if result.Key != tt.entry.Key {
				t.Errorf("Key = %v, want %v", result.Key, tt.entry.Key)
			}

			if result.Description != tt.entry.Description {
				t.Errorf("Description = %v, want %v", result.Description, tt.entry.Description)
			}

			if len(result.Localizations) != len(tt.entry.Localizations) {
				t.Errorf("Localizations length = %v, want %v", len(result.Localizations), len(tt.entry.Localizations))
			}

			if len(result.Scopes) != len(tt.entry.Scopes) {
				t.Errorf("Scopes length = %v, want %v", len(result.Scopes), len(tt.entry.Scopes))
			}
		})
	}
}

func TestLocalizationToProto(t *testing.T) {
	now := time.Now()
	userID := "user-123"

	tests := []struct {
		name          string
		localization  *models.Localization
		wantNil       bool
		expectedEmpty bool
	}{
		{
			name:         "nil localization",
			localization: nil,
			wantNil:      true,
		},
		{
			name: "basic localization",
			localization: &models.Localization{
				ID:          "loc-1",
				EntryID:     "entry-1",
				LocaleID:    "en",
				Translation: "Hello",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			wantNil: false,
		},
		{
			name: "localization with user tracking",
			localization: &models.Localization{
				ID:          "loc-1",
				EntryID:     "entry-1",
				LocaleID:    "en",
				Translation: "Hello",
				CreatedBy:   &userID,
				UpdatedBy:   &userID,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LocalizationToProto(tt.localization)

			if tt.wantNil {
				if result != nil {
					t.Errorf("LocalizationToProto() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("LocalizationToProto() returned nil")
			}

			if result.Id != tt.localization.ID {
				t.Errorf("ID = %v, want %v", result.Id, tt.localization.ID)
			}

			if result.EntryId != tt.localization.EntryID {
				t.Errorf("EntryID = %v, want %v", result.EntryId, tt.localization.EntryID)
			}

			if result.LocaleId != tt.localization.LocaleID {
				t.Errorf("LocaleID = %v, want %v", result.LocaleId, tt.localization.LocaleID)
			}

			if result.Translation != tt.localization.Translation {
				t.Errorf("Translation = %v, want %v", result.Translation, tt.localization.Translation)
			}
		})
	}
}

func TestEntriesToProto(t *testing.T) {
	now := time.Now()

	entries := []models.Entry{
		{
			ID:          "entry-1",
			Key:         "button.submit",
			Description: "Submit button",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "entry-2",
			Key:         "button.cancel",
			Description: "Cancel button",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	result := EntriesToProto(entries, "en")

	if len(result) != 2 {
		t.Fatalf("EntriesToProto() length = %v, want 2", len(result))
	}

	if result[0].Key != "button.submit" {
		t.Errorf("First entry key = %v, want button.submit", result[0].Key)
	}

	if result[1].Key != "button.cancel" {
		t.Errorf("Second entry key = %v, want button.cancel", result[1].Key)
	}
}

func TestLocalizationsToProto(t *testing.T) {
	now := time.Now()

	localizations := []models.Localization{
		{
			ID:          "loc-1",
			EntryID:     "entry-1",
			LocaleID:    "en",
			Translation: "Hello",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "loc-2",
			EntryID:     "entry-1",
			LocaleID:    "de",
			Translation: "Hallo",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	result := LocalizationsToProto(localizations)

	if len(result) != 2 {
		t.Fatalf("LocalizationsToProto() length = %v, want 2", len(result))
	}

	if result[0].Translation != "Hello" {
		t.Errorf("First translation = %v, want Hello", result[0].Translation)
	}

	if result[1].Translation != "Hallo" {
		t.Errorf("Second translation = %v, want Hallo", result[1].Translation)
	}
}
