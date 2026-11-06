package apitoken_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/apitoken"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestAuthenticate(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	owner := models.User{Email: "owner@example.com", Name: "Owner", PasswordHash: "-"}
	gone := models.User{Email: "gone@example.com", Name: "Gone", PasswordHash: "-"}

	for _, user := range []*models.User{&owner, &gone} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	add := func(user models.User, expires *time.Time) (string, *models.APIToken) {
		t.Helper()

		secret, hash, shown, err := apitoken.Generate()
		if err != nil {
			t.Fatal(err)
		}

		token := &models.APIToken{UserID: user.ID, Name: "CI", TokenHash: hash, Prefix: shown, ExpiresAt: expires}
		if err := db.Create(token).Error; err != nil {
			t.Fatal(err)
		}

		return secret, token
	}

	later := now.Add(time.Hour)
	valid, token := add(owner, nil)
	current, _ := add(owner, &later)
	expired, _ := add(owner, &now)
	deleted, _ := add(gone, nil)

	if err := db.Delete(&gone).Error; err != nil {
		t.Fatal(err)
	}

	at := now

	authenticator := apitoken.NewAuthenticator(db.DB)
	apitoken.SetNow(authenticator, func() time.Time { return at })

	ctx := context.Background()
	// use authenticates with the valid token at the time
	use := func(when time.Time) {
		t.Helper()

		at = when

		tokenID, userID, err := authenticator.Authenticate(ctx, valid)
		if err != nil || tokenID != token.ID || userID != owner.ID {
			t.Fatalf("Authenticate() = %q, %q, %v; want %q, %q", tokenID, userID, err, token.ID, owner.ID)
		}
	}
	lastUsed := func() time.Time {
		t.Helper()

		var row models.APIToken
		if err := db.First(&row, "id = ?", token.ID).Error; err != nil || row.LastUsedAt == nil {
			t.Fatalf("last used = %v, %v", row.LastUsedAt, err)
		}

		return *row.LastUsedAt
	}

	use(now)

	if !lastUsed().Equal(now) {
		t.Errorf("last used at %v, want %v", lastUsed(), now)
	}

	// The last use is recorded every five minutes at most.
	use(now.Add(4 * time.Minute))

	if !lastUsed().Equal(now) {
		t.Errorf("last used at %v after 4 minutes, want still %v", lastUsed(), now)
	}

	use(now.Add(5 * time.Minute))

	if !lastUsed().Equal(at) {
		t.Errorf("last used at %v after 5 minutes, want %v", lastUsed(), at)
	}

	at = now

	if _, userID, err := authenticator.Authenticate(ctx, current); err != nil || userID != owner.ID {
		t.Errorf("unexpired token: %q, %v", userID, err)
	}

	for name, secret := range map[string]string{"expired": expired, "deleted user": deleted, "unknown": "plc_unknown"} {
		if _, _, err := authenticator.Authenticate(ctx, secret); !errors.Is(err, apitoken.ErrInvalid) {
			t.Errorf("%s token: error = %v, want ErrInvalid", name, err)
		}
	}
}
