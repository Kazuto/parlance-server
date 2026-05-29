package converter

import (
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
)

func TestUserToProto(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		user    *models.User
		wantNil bool
	}{
		{
			name:    "nil user",
			user:    nil,
			wantNil: true,
		},
		{
			name: "basic user",
			user: &models.User{
				ID:        "user-id",
				Email:     "test@example.com",
				Name:      "Test User",
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantNil: false,
		},
		{
			name: "user with roles",
			user: &models.User{
				ID:        "user-id",
				Email:     "test@example.com",
				Name:      "Test User",
				CreatedAt: now,
				UpdatedAt: now,
				Roles: []models.Role{
					{
						ID:   "role-1",
						Name: "admin",
					},
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UserToProto(tt.user)

			if tt.wantNil {
				if result != nil {
					t.Errorf("UserToProto() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("UserToProto() returned nil, want non-nil")
			}

			if result.Id != tt.user.ID {
				t.Errorf("ID = %v, want %v", result.Id, tt.user.ID)
			}

			if result.Email != tt.user.Email {
				t.Errorf("Email = %v, want %v", result.Email, tt.user.Email)
			}

			if result.Name != tt.user.Name {
				t.Errorf("Name = %v, want %v", result.Name, tt.user.Name)
			}

			if len(result.Roles) != len(tt.user.Roles) {
				t.Errorf("Roles length = %v, want %v", len(result.Roles), len(tt.user.Roles))
			}
		})
	}
}
