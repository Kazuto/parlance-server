package middleware

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
)

// HasPermission checks if the user in the context has a specific permission
func HasPermission(ctx context.Context, db *database.DB, permissionName string) bool {
	userID, ok := ctx.Value(ContextKeyUserID).(string)
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

// RequirePermission checks permission and returns an error if not authorized
func RequirePermission(ctx context.Context, db *database.DB, permissionName string) error {
	if !HasPermission(ctx, db, permissionName) {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("permission '%s' required", permissionName))
	}
	return nil
}
