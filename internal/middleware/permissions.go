package middleware

import (
	"context"
	"fmt"

	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
)

const contextKeyPermissions ContextKey = "permissions"

// permissionSet holds the names of the signed-in user's permissions
type permissionSet map[string]bool

// WithPermissions loads the signed-in user's permissions into the context, so later checks
// in the same request need no query. Without a user the context is returned unchanged.
func WithPermissions(ctx context.Context, db *database.DB) (context.Context, error) {
	userID := UserID(ctx)
	if userID == "" {
		return ctx, nil
	}

	if _, ok := ctx.Value(contextKeyPermissions).(permissionSet); ok {
		return ctx, nil
	}

	var names []string
	if err := db.WithContext(ctx).
		Model(&models.Permission{}).
		Distinct("permissions.name").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Joins("JOIN roles ON roles.id = role_permissions.role_id AND roles.deleted_at IS NULL").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Joins("JOIN users ON users.id = user_roles.user_id AND users.deleted_at IS NULL").
		Where("users.id = ?", userID).
		Pluck("permissions.name", &names).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load permissions")
	}

	set := make(permissionSet, len(names))
	for _, name := range names {
		set[name] = true
	}

	return context.WithValue(ctx, contextKeyPermissions, set), nil
}

// HasPermission checks if the user in the context has a specific permission, using the
// permissions WithPermissions loaded when present
func HasPermission(ctx context.Context, db *database.DB, permissionName string) bool {
	ctx, err := WithPermissions(ctx, db)
	if err != nil {
		return false
	}

	set, ok := ctx.Value(contextKeyPermissions).(permissionSet)

	return ok && set[permissionName]
}

// RequirePermission checks permission and returns an error if not authorized
func RequirePermission(ctx context.Context, db *database.DB, permissionName string) error {
	if !HasPermission(ctx, db, permissionName) {
		return errors.PermissionDenied(errors.ErrPermissionDenied, fmt.Sprintf("permission '%s' required", permissionName))
	}

	return nil
}
