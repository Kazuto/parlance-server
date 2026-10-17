package middleware

import (
	"context"

	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
)

// HasPermission checks if the user in the context has a specific permission
func HasPermission(ctx context.Context, db *database.DB, permissionName string) bool {
	userID, ok := ctx.Value("user_id").(string)
	if !ok || userID == "" {
		return false
	}

	var user models.User
	if err := db.Preload("Roles.Permissions").First(&user, "id = ?", userID).Error; err != nil {
		return false
	}

	// Check if any of the user's roles has the required permission
	for _, role := range user.Roles {
		for _, permission := range role.Permissions {
			if permission.Name == permissionName {
				return true
			}
		}
	}

	return false
}
