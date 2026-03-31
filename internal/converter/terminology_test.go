package converter

import (
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
)

func TestTerminologyToProto(t *testing.T) {
	now := time.Now()
	userID := "user-123"

	tests := []struct {
		name        string
		terminology *models.Terminology
		wantNil     bool
	}{
		{
			name:        "nil terminology",
			terminology: nil,
			wantNil:     true,
		},
		{
			name: "basic terminology",
			terminology: &models.Terminology{
				ID:          "term-1",
				Term:        "API",
				Description: "Application Programming Interface",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			wantNil: false,
		},
		{
			name: "terminology with user tracking",
			terminology: &models.Terminology{
				ID:          "term-1",
				Term:        "API",
				Description: "Application Programming Interface",
				CreatedBy:   &userID,
				UpdatedBy:   &userID,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			wantNil: false,
		},
		{
			name: "terminology with definitions",
			terminology: &models.Terminology{
				ID:          "term-1",
				Term:        "API",
				Description: "Application Programming Interface",
				CreatedAt:   now,
				UpdatedAt:   now,
				Definitions: []models.Definition{
					{
						ID:            "def-1",
						TerminologyID: "term-1",
						LocaleID:      "en",
						Translation:   "Always use 'API' - do not translate",
						CreatedAt:     now,
						UpdatedAt:     now,
					},
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TerminologyToProto(tt.terminology)

			if tt.wantNil {
				if result != nil {
					t.Errorf("TerminologyToProto() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("TerminologyToProto() returned nil")
			}

			if result.Id != tt.terminology.ID {
				t.Errorf("ID = %v, want %v", result.Id, tt.terminology.ID)
			}

			if result.Term != tt.terminology.Term {
				t.Errorf("Term = %v, want %v", result.Term, tt.terminology.Term)
			}

			if result.Description != tt.terminology.Description {
				t.Errorf("Description = %v, want %v", result.Description, tt.terminology.Description)
			}

			if len(result.Definitions) != len(tt.terminology.Definitions) {
				t.Errorf("Definitions length = %v, want %v", len(result.Definitions), len(tt.terminology.Definitions))
			}

			if tt.terminology.CreatedBy != nil {
				if result.CreatedBy != *tt.terminology.CreatedBy {
					t.Errorf("CreatedBy = %v, want %v", result.CreatedBy, *tt.terminology.CreatedBy)
				}
			} else {
				if result.CreatedBy != "" {
					t.Errorf("CreatedBy = %v, want empty", result.CreatedBy)
				}
			}
		})
	}
}

func TestTerminologiesToProto(t *testing.T) {
	now := time.Now()

	terminologies := []models.Terminology{
		{
			ID:          "term-1",
			Term:        "API",
			Description: "Application Programming Interface",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "term-2",
			Term:        "REST",
			Description: "Representational State Transfer",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	result := TerminologiesToProto(terminologies)

	if len(result) != 2 {
		t.Fatalf("TerminologiesToProto() length = %v, want 2", len(result))
	}

	if result[0].Term != "API" {
		t.Errorf("First term = %v, want API", result[0].Term)
	}

	if result[1].Term != "REST" {
		t.Errorf("Second term = %v, want REST", result[1].Term)
	}
}
