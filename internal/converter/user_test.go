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
