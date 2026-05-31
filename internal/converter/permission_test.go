package converter

import (
	"testing"

	"github.com/kazuto/parlance-server/internal/models"
)

func TestPermissionToProto(t *testing.T) {
	tests := []struct {
		name       string
		permission *models.Permission
		wantNil    bool
	}{
		{
			name:       "nil permission",
			permission: nil,
			wantNil:    true,
		},
		{
			name: "basic permission",
			permission: &models.Permission{
				ID:       "perm-id",
				Name:     "create_user",
				Resource: "user",
				Action:   "create",
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PermissionToProto(tt.permission)

			if tt.wantNil {
				if result != nil {
					t.Errorf("PermissionToProto() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("PermissionToProto() returned nil")
			}

			if result.Id != tt.permission.ID {
				t.Errorf("ID = %v, want %v", result.Id, tt.permission.ID)
			}

			if result.Name != tt.permission.Name {
				t.Errorf("Name = %v, want %v", result.Name, tt.permission.Name)
			}

			if result.Resource != tt.permission.Resource {
				t.Errorf("Resource = %v, want %v", result.Resource, tt.permission.Resource)
			}

			if result.Action != tt.permission.Action {
				t.Errorf("Action = %v, want %v", result.Action, tt.permission.Action)
			}
		})
	}
}

func TestPermissionsToProto(t *testing.T) {
	permissions := []models.Permission{
		{
			ID:       "perm-1",
			Name:     "create_user",
			Resource: "user",
			Action:   "create",
		},
		{
			ID:       "perm-2",
			Name:     "read_user",
			Resource: "user",
			Action:   "read",
		},
	}

	result := PermissionsToProto(permissions)

	if len(result) != 2 {
		t.Fatalf("PermissionsToProto() length = %v, want 2", len(result))
	}

	if result[0].Name != "create_user" {
		t.Errorf("First permission name = %v, want create_user", result[0].Name)
	}

	if result[1].Name != "read_user" {
		t.Errorf("Second permission name = %v, want read_user", result[1].Name)
	}
}
