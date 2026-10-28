package common

import (
	"context"
	stderrors "errors"

	"connectrpc.com/connect"

	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

// NewUser holds what creating a user needs
type NewUser struct {
	Email    string
	Name     string
	Password string
	Roles    []models.Role
	// DefaultRole names a role every new user gets when it exists, e.g. "viewer"
	DefaultRole string
	// External users sign in with a single sign-on provider and have no password.
	External bool
}

// CreateUser creates a user with an unused email, returning it with its roles and their
// permissions loaded. Everything runs in one transaction.
func CreateUser(ctx context.Context, db *gorm.DB, input NewUser) (*models.User, error) {
	if input.Email == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "email is required")
	}

	if input.Password == "" && !input.External {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "password is required")
	}

	if input.Name == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "name is required")
	}

	// Without a hash, no password matches.
	hashedPassword := ""

	if !input.External {
		var err error
		if hashedPassword, err = auth.HashPassword(input.Password); err != nil {
			return nil, errors.InternalCause(err, "failed to hash password")
		}
	}

	user := &models.User{
		Email:        input.Email,
		PasswordHash: hashedPassword,
		Name:         input.Name,
		Roles:        input.Roles,
	}

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		exists, err := Exists(tx.Model(&models.User{}).Where("email = ?", input.Email))
		if err != nil {
			return err
		}

		if exists {
			return errors.AlreadyExists(errors.ErrUserAlreadyExists, "user with this email already exists")
		}

		if input.DefaultRole != "" {
			var role models.Role
			err := tx.Where("name = ?", input.DefaultRole).First(&role).Error

			switch {
			case err == nil:
				if !hasRole(user.Roles, role.ID) {
					user.Roles = append(user.Roles, role)
				}
			case !stderrors.Is(err, gorm.ErrRecordNotFound):
				return err
			}
		}

		if err := tx.Create(user).Error; err != nil {
			return err
		}

		return tx.Preload("Roles.Permissions").First(user, "id = ?", user.ID).Error
	})
	if err != nil {
		if connect.CodeOf(err) != connect.CodeUnknown {
			return nil, err
		}

		return nil, errors.InternalCause(err, "failed to create user")
	}

	return user, nil
}

func hasRole(roles []models.Role, id string) bool {
	for _, role := range roles {
		if role.ID == id {
			return true
		}
	}

	return false
}
