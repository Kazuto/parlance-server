package converter

import (
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
)

func TestScopeToProto(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		scope   *models.Scope
		wantNil bool
	}{
		{
			name:    "nil scope",
			scope:   nil,
			wantNil: true,
		},
		{
			name: "basic scope",
			scope: &models.Scope{
				ID:          "scope-id",
				Name:        "admin",
				Description: "Admin scope",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ScopeToProto(tt.scope)

			if tt.wantNil {
				if result != nil {
					t.Errorf("ScopeToProto() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("ScopeToProto() returned nil")
			}

			if result.Id != tt.scope.ID {
				t.Errorf("ID = %v, want %v", result.Id, tt.scope.ID)
			}

			if result.Name != tt.scope.Name {
				t.Errorf("Name = %v, want %v", result.Name, tt.scope.Name)
			}

			if result.Description != tt.scope.Description {
				t.Errorf("Description = %v, want %v", result.Description, tt.scope.Description)
			}
		})
	}
}

func TestScopesToProto(t *testing.T) {
	now := time.Now()
	scopes := []models.Scope{
		{
			ID:          "scope-1",
			Name:        "admin",
			Description: "Admin scope",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "scope-2",
			Name:        "public",
			Description: "Public scope",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	result := ScopesToProto(scopes)

	if len(result) != 2 {
		t.Fatalf("ScopesToProto() length = %v, want 2", len(result))
	}

	if result[0].Name != "admin" {
		t.Errorf("First scope name = %v, want admin", result[0].Name)
	}

	if result[1].Name != "public" {
		t.Errorf("Second scope name = %v, want public", result[1].Name)
	}
}
