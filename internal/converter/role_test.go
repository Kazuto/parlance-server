package converter

import (
	"testing"

	"github.com/kazuto/parlance-server/internal/models"
)

func TestRoleToProto(t *testing.T) {
	tests := []struct {
		name    string
		role    *models.Role
		wantNil bool
	}{
		{
			name:    "nil role",
			role:    nil,
			wantNil: true,
		},
		{
			name: "basic role",
			role: &models.Role{
				ID:   "role-id",
				Name: "admin",
			},
			wantNil: false,
		},
		{
			name: "role with permissions",
			role: &models.Role{
				ID:   "role-id",
				Name: "admin",
				Permissions: []models.Permission{
					{
						ID:       "perm-1",
						Name:     "create_user",
						Resource: "user",
						Action:   "create",
					},
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoleToProto(tt.role)

			if tt.wantNil {
				if result != nil {
					t.Errorf("RoleToProto() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("RoleToProto() returned nil")
			}

			if result.Id != tt.role.ID {
				t.Errorf("ID = %v, want %v", result.Id, tt.role.ID)
			}

			if result.Name != tt.role.Name {
				t.Errorf("Name = %v, want %v", result.Name, tt.role.Name)
			}

			if len(result.Permissions) != len(tt.role.Permissions) {
				t.Errorf("Permissions length = %v, want %v", len(result.Permissions), len(tt.role.Permissions))
			}
		})
	}
}

func TestRolesToProto(t *testing.T) {
	roles := []models.Role{
		{
			ID:   "role-1",
			Name: "admin",
		},
		{
			ID:   "role-2",
			Name: "viewer",
		},
	}

	result := RolesToProto(roles)

	if len(result) != 2 {
		t.Fatalf("RolesToProto() length = %v, want 2", len(result))
	}

	if result[0].Name != "admin" {
		t.Errorf("First role name = %v, want admin", result[0].Name)
	}

	if result[1].Name != "viewer" {
		t.Errorf("Second role name = %v, want viewer", result[1].Name)
	}
}
