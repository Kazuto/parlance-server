package converter

import (
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
)

func TestDefinitionToProto(t *testing.T) {
	now := time.Now()
	userID := "user-123"

	tests := []struct {
		name       string
		definition *models.Definition
		wantNil    bool
	}{
		{
			name:       "nil definition",
			definition: nil,
			wantNil:    true,
		},
		{
			name: "basic definition",
			definition: &models.Definition{
				ID:            "def-1",
				TerminologyID: "term-1",
				LocaleID:      "en",
				Translation:   "Use 'API' untranslated",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			wantNil: false,
		},
		{
			name: "definition with user tracking",
			definition: &models.Definition{
				ID:            "def-1",
				TerminologyID: "term-1",
				LocaleID:      "en",
				Translation:   "Use 'API' untranslated",
				CreatedBy:     &userID,
				UpdatedBy:     &userID,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DefinitionToProto(tt.definition)

			if tt.wantNil {
				if result != nil {
					t.Errorf("DefinitionToProto() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("DefinitionToProto() returned nil")
			}

			if result.Id != tt.definition.ID {
				t.Errorf("ID = %v, want %v", result.Id, tt.definition.ID)
			}

			if result.TerminologyId != tt.definition.TerminologyID {
				t.Errorf("TerminologyID = %v, want %v", result.TerminologyId, tt.definition.TerminologyID)
			}

			if result.LocaleId != tt.definition.LocaleID {
				t.Errorf("LocaleID = %v, want %v", result.LocaleId, tt.definition.LocaleID)
			}

			if result.Translation != tt.definition.Translation {
				t.Errorf("Translation = %v, want %v", result.Translation, tt.definition.Translation)
			}

			if tt.definition.CreatedBy != nil {
				if result.CreatedBy != *tt.definition.CreatedBy {
					t.Errorf("CreatedBy = %v, want %v", result.CreatedBy, *tt.definition.CreatedBy)
				}
			} else {
				if result.CreatedBy != "" {
					t.Errorf("CreatedBy = %v, want empty", result.CreatedBy)
				}
			}
		})
	}
}

func TestDefinitionsToProto(t *testing.T) {
	now := time.Now()

	definitions := []models.Definition{
		{
			ID:            "def-1",
			TerminologyID: "term-1",
			LocaleID:      "en",
			Translation:   "Use 'API' untranslated",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			ID:            "def-2",
			TerminologyID: "term-1",
			LocaleID:      "de",
			Translation:   "Verwenden Sie 'API' unübersetzt",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}

	result := DefinitionsToProto(definitions)

	if len(result) != 2 {
		t.Fatalf("DefinitionsToProto() length = %v, want 2", len(result))
	}

	if result[0].LocaleId != "en" {
		t.Errorf("First definition locale = %v, want en", result[0].LocaleId)
	}

	if result[1].LocaleId != "de" {
		t.Errorf("Second definition locale = %v, want de", result[1].LocaleId)
	}
}

func TestDefinitionHistoryToProto(t *testing.T) {
	now := time.Now()
	userID := "user-123"

	tests := []struct {
		name    string
		history *models.DefinitionHistory
		wantNil bool
	}{
		{
			name:    "nil history",
			history: nil,
			wantNil: true,
		},
		{
			name: "basic history",
			history: &models.DefinitionHistory{
				ID:            "hist-1",
				DefinitionID:  "def-1",
				LocaleID:      "en",
				TerminologyID: "term-1",
				Translation:   "Previous translation",
				Action:        "updated",
				ChangedAt:     now,
			},
			wantNil: false,
		},
		{
			name: "history with user",
			history: &models.DefinitionHistory{
				ID:            "hist-1",
				DefinitionID:  "def-1",
				LocaleID:      "en",
				TerminologyID: "term-1",
				UserID:        &userID,
				Translation:   "Previous translation",
				Action:        "updated",
				ChangedAt:     now,
				User: &models.User{
					ID:    userID,
					Email: "test@example.com",
					Name:  "Test User",
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DefinitionHistoryToProto(tt.history)

			if tt.wantNil {
				if result != nil {
					t.Errorf("DefinitionHistoryToProto() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("DefinitionHistoryToProto() returned nil")
			}

			if result.Id != tt.history.ID {
				t.Errorf("ID = %v, want %v", result.Id, tt.history.ID)
			}

			if result.DefinitionId != tt.history.DefinitionID {
				t.Errorf("DefinitionID = %v, want %v", result.DefinitionId, tt.history.DefinitionID)
			}

			if result.Action != tt.history.Action {
				t.Errorf("Action = %v, want %v", result.Action, tt.history.Action)
			}

			if tt.history.UserID != nil {
				if result.UserId != *tt.history.UserID {
					t.Errorf("UserID = %v, want %v", result.UserId, *tt.history.UserID)
				}
			} else {
				if result.UserId != "" {
					t.Errorf("UserID = %v, want empty", result.UserId)
				}
			}
		})
	}
}

func TestDefinitionHistoriesToProto(t *testing.T) {
	now := time.Now()
	userID := "user-123"

	histories := []models.DefinitionHistory{
		{
			ID:            "hist-1",
			DefinitionID:  "def-1",
			LocaleID:      "en",
			TerminologyID: "term-1",
			UserID:        &userID,
			Translation:   "First version",
			Action:        "created",
			ChangedAt:     now,
		},
		{
			ID:            "hist-2",
			DefinitionID:  "def-1",
			LocaleID:      "en",
			TerminologyID: "term-1",
			UserID:        &userID,
			Translation:   "Second version",
			Action:        "updated",
			ChangedAt:     now.Add(time.Hour),
		},
	}

	result := DefinitionHistoriesToProto(histories)

	if len(result) != 2 {
		t.Fatalf("DefinitionHistoriesToProto() length = %v, want 2", len(result))
	}

	if result[0].Action != "created" {
		t.Errorf("First action = %v, want created", result[0].Action)
	}

	if result[1].Action != "updated" {
		t.Errorf("Second action = %v, want updated", result[1].Action)
	}
}
