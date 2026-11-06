package sso

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/sso/ssotest"
	"gorm.io/gorm"
)

const redirectURI = "http://app.test/login/sso"

type fixture struct {
	db       *gorm.DB
	client   *Client
	idp      *ssotest.Provider
	provider models.SSOProvider
	viewer   models.Role
}

// setup starts a provider set up in Parlance with the viewer role for new accounts.
func setup(t *testing.T) *fixture {
	t.Helper()

	_, db := testutil.Setup(t)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM sso_logins`)
		db.Exec(`DELETE FROM sso_identities`)
		db.Exec(`DELETE FROM sso_providers`)
		db.Exec(`DELETE FROM users WHERE email <> 'test@example.com'`)
		db.Exec(`DELETE FROM roles WHERE name = 'viewer'`)
	})

	box, err := secret.New("test key")
	if err != nil {
		t.Fatal(err)
	}

	f := &fixture{db: db.DB, client: New(db.DB, box, nil), idp: ssotest.New(t)}

	f.viewer = models.Role{ID: uuid.NewString(), Name: "viewer"}
	if err := db.Create(&f.viewer).Error; err != nil {
		t.Fatal(err)
	}

	encrypted, err := box.Encrypt(f.idp.ClientSecret)
	if err != nil {
		t.Fatal(err)
	}

	f.provider = models.SSOProvider{
		Name: "Test IdP", IssuerURL: f.idp.Issuer(), ClientID: f.idp.ClientID, ClientSecret: encrypted,
		DefaultRoleID: &f.viewer.ID, Enabled: true,
	}
	if err := db.Create(&f.provider).Error; err != nil {
		t.Fatal(err)
	}

	return f
}

// signIn runs a whole sign-in as the provider's Account.
func (f *fixture) signIn(t *testing.T) (*models.User, error) {
	t.Helper()

	authorizationURL, err := f.client.Start(context.Background(), f.provider.ID, redirectURI)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	state, code := f.idp.SignIn(t, authorizationURL)

	return f.client.Finish(context.Background(), state, code)
}

// expectCode fails the test unless err carries the error code.
func expectCode(t *testing.T, err error, code string) {
	t.Helper()

	if err == nil || !strings.Contains(err.Error(), "["+code+"]") {
		t.Errorf("error = %v, want %s", err, code)
	}
}

func TestStartSendsToTheProvider(t *testing.T) {
	f := setup(t)

	authorizationURL, err := f.client.Start(context.Background(), f.provider.ID, redirectURI)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	parsed, _ := url.Parse(authorizationURL)
	query := parsed.Query()

	if parsed.Path != "/auth" || query.Get("redirect_uri") != redirectURI || query.Get("scope") != "openid profile email" ||
		query.Get("nonce") == "" || query.Get("code_challenge") == "" {
		t.Errorf("authorization url = %s", authorizationURL)
	}

	var login models.SSOLogin
	if err := f.db.First(&login, "state = ?", query.Get("state")).Error; err != nil {
		t.Fatalf("no sign-in stored for the state: %v", err)
	}

	if login.Nonce != query.Get("nonce") || login.RedirectURI != redirectURI || time.Until(login.ExpiresAt) > loginTTL {
		t.Errorf("stored sign-in = %+v", login)
	}

	f.db.Model(&f.provider).Update("enabled", false)

	_, err = f.client.Start(context.Background(), f.provider.ID, redirectURI)
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("turned off provider: error = %v, want not found", err)
	}
}

func TestFirstSignInCreatesAnAccount(t *testing.T) {
	f := setup(t)
	f.idp.Account = map[string]any{"sub": "ada-1", "email": "ada@example.com", "email_verified": true, "name": "Ada Lovelace"}

	user, err := f.signIn(t)
	if err != nil {
		t.Fatalf("sign-in error = %v", err)
	}

	if user.Email != "ada@example.com" || user.Name != "Ada Lovelace" || user.PasswordHash != "" ||
		len(user.Roles) != 1 || user.Roles[0].ID != f.viewer.ID {
		t.Errorf("user = %+v, want a viewer without a password", user)
	}

	// Later sign-ins find the account by its id at the provider, whatever its email.
	f.idp.Account["email"] = "ada@lovelace.example"

	again, err := f.signIn(t)
	if err != nil || again.ID != user.ID {
		t.Fatalf("second sign-in = %+v, %v; want the same user", again, err)
	}

	var identity models.SSOIdentity

	f.db.First(&identity, "subject = ?", "ada-1")

	if identity.UserID != user.ID || identity.Email != "ada@lovelace.example" {
		t.Errorf("identity = %+v", identity)
	}
}

func TestSignInLinksTheUserWithTheEmail(t *testing.T) {
	f := setup(t)
	// testutil's user, whose address differs in case.
	f.idp.Account = map[string]any{"sub": "test-1", "email": "Test@Example.com", "email_verified": "true"}

	user, err := f.signIn(t)
	if err != nil {
		t.Fatalf("sign-in error = %v", err)
	}

	if user.Email != "test@example.com" || len(user.Roles) == 0 || len(user.Roles[0].Permissions) == 0 {
		t.Errorf("user = %+v, want the existing user with their permissions", user)
	}
}

func TestSignInTakesTheEmailFromUserInfo(t *testing.T) {
	f := setup(t)
	f.idp.Account = map[string]any{"sub": "grace-1", "preferred_username": "grace"}
	f.idp.UserInfo = map[string]any{"sub": "grace-1", "email": "grace@example.com", "email_verified": true}

	user, err := f.signIn(t)
	if err != nil || user.Email != "grace@example.com" || user.Name != "grace" {
		t.Errorf("user = %+v, %v", user, err)
	}
}

func TestSignInRefusals(t *testing.T) {
	t.Run("without a role for new accounts", func(t *testing.T) {
		f := setup(t)
		f.db.Model(&f.provider).Update("default_role_id", nil)
		f.idp.Account = map[string]any{"sub": "x", "email": "new@example.com", "email_verified": true}

		_, err := f.signIn(t)
		expectCode(t, err, "SSO_NO_ACCOUNT")
	})

	t.Run("with an unverified email", func(t *testing.T) {
		f := setup(t)
		f.idp.Account = map[string]any{"sub": "x", "email": "test@example.com", "email_verified": "false"}

		_, err := f.signIn(t)
		expectCode(t, err, "SSO_EMAIL_UNVERIFIED")
	})

	t.Run("without an email", func(t *testing.T) {
		f := setup(t)
		f.idp.Account = map[string]any{"sub": "x"}

		_, err := f.signIn(t)
		expectCode(t, err, "SSO_NO_EMAIL")
	})

	t.Run("for a deleted user", func(t *testing.T) {
		f := setup(t)
		f.idp.Account = map[string]any{"sub": "gone-1", "email": "gone@example.com"}

		user, err := f.signIn(t)
		if err != nil {
			t.Fatal(err)
		}

		f.db.Delete(user)

		_, err = f.signIn(t)
		expectCode(t, err, "SSO_ACCOUNT_DELETED")

		// Nor can another account at the provider take over the address.
		f.idp.Account = map[string]any{"sub": "gone-2", "email": "gone@example.com"}

		_, err = f.signIn(t)
		expectCode(t, err, "SSO_ACCOUNT_DELETED")
	})

	t.Run("with an ID token of another sign-in", func(t *testing.T) {
		f := setup(t)
		f.idp.Account = map[string]any{"sub": "x", "email": "test@example.com"}
		f.idp.Nonce = "someone else's"

		_, err := f.signIn(t)
		expectCode(t, err, "SSO_FAILED")
	})

	t.Run("with a wrong client secret", func(t *testing.T) {
		f := setup(t)
		f.idp.ClientSecret = "changed at the provider"
		f.idp.Account = map[string]any{"sub": "x", "email": "test@example.com"}

		_, err := f.signIn(t)
		expectCode(t, err, "SSO_FAILED")
	})

	t.Run("used twice or too late", func(t *testing.T) {
		f := setup(t)
		f.idp.Account = map[string]any{"sub": "x", "email": "test@example.com"}

		authorizationURL, err := f.client.Start(context.Background(), f.provider.ID, redirectURI)
		if err != nil {
			t.Fatal(err)
		}

		state, code := f.idp.SignIn(t, authorizationURL)

		if _, err := f.client.Finish(context.Background(), state, code); err != nil {
			t.Fatalf("first finish: %v", err)
		}

		_, err = f.client.Finish(context.Background(), state, code)
		expectCode(t, err, "SSO_LOGIN_EXPIRED")

		authorizationURL, _ = f.client.Start(context.Background(), f.provider.ID, redirectURI)
		state, code = f.idp.SignIn(t, authorizationURL)
		f.client.now = func() time.Time { return time.Now().Add(loginTTL + time.Minute) }

		_, err = f.client.Finish(context.Background(), state, code)
		expectCode(t, err, "SSO_LOGIN_EXPIRED")
	})
}

func TestCheck(t *testing.T) {
	idp := ssotest.New(t)
	client := New(nil, nil, nil)

	if err := client.Check(context.Background(), idp.Issuer()); err != nil {
		t.Errorf("Check() error = %v", err)
	}

	// The provider must name itself exactly as configured.
	if err := client.Check(context.Background(), idp.Issuer()+"/"); err == nil {
		t.Error("an issuer with another spelling passed")
	}
}
