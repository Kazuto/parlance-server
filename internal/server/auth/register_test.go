package auth

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/settings"
)

func TestRegisterWhileRegistrationIsOpen(t *testing.T) {
	_, db := testutil.Setup(t)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM settings`)
		db.Exec(`DELETE FROM users WHERE email LIKE 'new%'`)
	})

	server := NewServer(db, &config.Config{JWT: config.JWTConfig{Secret: "jwt-secret", Expiry: "1h"}}, nil)
	register := func(email string) error {
		_, err := server.Register(context.Background(), connect.NewRequest(&pb.RegisterRequest{
			Email: email, Password: "password", Name: "New",
		}))

		return err
	}

	// Open until an admin closes it
	if err := register("new@example.com"); err != nil {
		t.Fatalf("Register() while open: %v", err)
	}

	if err := settings.SetBool(db.DB, models.SettingRegistrationOpen, false, nil); err != nil {
		t.Fatal(err)
	}

	err := register("new2@example.com")
	if connect.CodeOf(err) != connect.CodePermissionDenied || !strings.Contains(err.Error(), errors.ErrRegistrationClosed) {
		t.Errorf("Register() while closed: %v, want registration closed", err)
	}

	var count int64
	if db.Model(&models.User{}).Where("email = ?", "new2@example.com").Count(&count); count != 0 {
		t.Error("the account was created while registration was closed")
	}
}
