package common

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestCreateUser(t *testing.T) {
	_, db := testutil.Setup(t)
	viewer := models.Role{Name: "viewer"}
	editor := models.Role{Name: "editor"}

	for _, role := range []*models.Role{&viewer, &editor} {
		if err := db.Create(role).Error; err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()

	user, err := CreateUser(ctx, db.DB, NewUser{
		Email: "new@example.com", Name: "New", Password: "secret123",
		Roles: []models.Role{editor, viewer}, DefaultRole: "viewer",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if len(user.Roles) != 2 {
		t.Errorf("user has %d roles, want editor and viewer once each", len(user.Roles))
	}

	if user.PasswordHash == "" || user.PasswordHash == "secret123" {
		t.Error("password was not hashed")
	}

	_, err = CreateUser(ctx, db.DB, NewUser{Email: "new@example.com", Name: "Again", Password: "secret123"})
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("duplicate email: got %v, want already exists", err)
	}

	_, err = CreateUser(ctx, db.DB, NewUser{Email: "other@example.com", Name: "Other"})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("missing password: got %v, want invalid argument", err)
	}

	// A missing default role is skipped.
	user, err = CreateUser(ctx, db.DB, NewUser{Email: "plain@example.com", Name: "Plain", Password: "secret123", DefaultRole: "nonexistent"})
	if err != nil || len(user.Roles) != 0 {
		t.Errorf("unknown default role: got %v roles, error %v", len(user.Roles), err)
	}
}
