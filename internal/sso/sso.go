// Package sso signs users in with OpenID Connect providers. Start sends them to
// the provider with a fresh state, nonce and PKCE verifier; Finish takes the code
// the provider sends back, verifies the ID token, and finds, links or creates the
// user's Parlance account.
package sso

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/common"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

const (
	// loginTTL is how long someone may take at the provider's sign-in page.
	loginTTL = 10 * time.Minute
	// discoveryTTL is how long a provider's configuration and keys are reused.
	discoveryTTL = time.Hour
)

// scopes ask for the account's id, name and email.
var scopes = []string{oidc.ScopeOpenID, "profile", "email"}

// Client signs users in with the providers stored in the database.
type Client struct {
	db   *gorm.DB
	box  *secret.Box
	http *http.Client
	// now tells the time; tests replace it.
	now func() time.Time

	mu        sync.Mutex
	providers map[string]cachedProvider
}

// cachedProvider is a provider's discovered configuration, with the signing keys
// it fetched since.
type cachedProvider struct {
	provider  *oidc.Provider
	fetchedAt time.Time
}

// New creates a client that decrypts client secrets with the box and talks to
// providers with the HTTP client, nil for one with a timeout.
func New(db *gorm.DB, box *secret.Box, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	return &Client{db: db, box: box, http: client, now: time.Now, providers: map[string]cachedProvider{}}
}

// Check fetches the provider configuration of the issuer, which also checks that
// the issuer names itself the same way.
func (c *Client) Check(ctx context.Context, issuerURL string) error {
	_, err := oidc.NewProvider(oidc.ClientContext(ctx, c.http), issuerURL)

	return err
}

// Start begins a sign-in with the provider and returns the provider's page to send
// the browser to. It comes back to redirectURI with a code and the state.
func (c *Client) Start(ctx context.Context, providerID, redirectURI string) (string, error) {
	provider, err := c.enabledProvider(ctx, providerID)
	if err != nil {
		return "", err
	}

	config, _, err := c.connect(provider, redirectURI)
	if err != nil {
		return "", err
	}

	login := models.SSOLogin{
		State:        randomString(),
		ProviderID:   provider.ID,
		Nonce:        randomString(),
		CodeVerifier: oauth2.GenerateVerifier(),
		RedirectURI:  redirectURI,
		ExpiresAt:    c.now().Add(loginTTL),
	}

	err = c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Sign-ins nobody finished are dropped along the way.
		if err := tx.Where("expires_at < ?", c.now()).Delete(&models.SSOLogin{}).Error; err != nil {
			return err
		}

		return tx.Create(&login).Error
	})
	if err != nil {
		return "", errors.InternalCause(err, "failed to start the sign-in")
	}

	return config.AuthCodeURL(login.State, oidc.Nonce(login.Nonce), oauth2.S256ChallengeOption(login.CodeVerifier)), nil
}

// Finish completes the sign-in the state belongs to with the code the provider
// sent back, and returns the user, with their roles and permissions.
func (c *Client) Finish(ctx context.Context, state, code string) (*models.User, error) {
	login, err := c.takeLogin(ctx, state)
	if err != nil {
		return nil, err
	}

	provider, err := c.enabledProvider(ctx, login.ProviderID)
	if err != nil {
		return nil, err
	}

	config, discovered, err := c.connect(provider, login.RedirectURI)
	if err != nil {
		return nil, err
	}

	ctx = oidc.ClientContext(ctx, c.http)

	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(login.CodeVerifier))
	if err != nil {
		return nil, failed("the provider turned down the sign-in", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, failed("the provider sent no ID token", nil)
	}

	idToken, err := discovered.VerifierContext(ctx, &oidc.Config{ClientID: provider.ClientID, Now: c.now}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, failed("the ID token is not valid", err)
	}

	if idToken.Nonce != login.Nonce {
		return nil, failed("the ID token belongs to another sign-in", nil)
	}

	var claims profile
	if err := idToken.Claims(&claims); err != nil {
		return nil, failed("the ID token can't be read", err)
	}

	// Some providers only share the email through the userinfo endpoint.
	if claims.Email == "" && discovered.UserInfoEndpoint() != "" {
		info, err := discovered.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if err != nil {
			return nil, failed("the provider's user info can't be read", err)
		}

		if err := info.Claims(&claims); err != nil {
			return nil, failed("the provider's user info can't be read", err)
		}
	}

	return c.signIn(ctx, &provider, idToken.Subject, claims)
}

// profile is what Parlance reads from the ID token or user info.
type profile struct {
	Email             string `json:"email"`
	EmailVerified     flag   `json:"email_verified"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
}

// flag reads a boolean claim, which some providers send as a string.
type flag struct {
	set, value bool
}

func (f *flag) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch value := raw.(type) {
	case bool:
		*f = flag{set: true, value: value}
	case string:
		*f = flag{set: true, value: strings.EqualFold(value, "true")}
	}

	return nil
}

// displayName is the name of an account created at the first sign-in.
func (p profile) displayName() string {
	for _, name := range []string{p.Name, p.PreferredUsername} {
		if name = strings.TrimSpace(name); name != "" {
			return name
		}
	}

	name, _, _ := strings.Cut(p.Email, "@")

	return name
}

// signIn finds the user the provider's account is linked to, or links it to the
// user with its email, or creates one when the provider has a default role.
func (c *Client) signIn(ctx context.Context, provider *models.SSOProvider, subject string, claims profile) (*models.User, error) {
	var userID string

	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := c.now()
		email := strings.TrimSpace(claims.Email)

		var identities []models.SSOIdentity
		if err := tx.Where("provider_id = ? AND subject = ?", provider.ID, subject).Limit(1).Find(&identities).Error; err != nil {
			return err
		}

		if len(identities) > 0 {
			identity := identities[0]
			if err := requireActive(tx, identity.UserID); err != nil {
				return err
			}

			userID = identity.UserID

			return tx.Model(&identity).Updates(map[string]any{"email": email, "last_login_at": now}).Error
		}

		// The first sign-in with this account goes by its email.
		if email == "" {
			return errors.PermissionDenied(errors.ErrSSONoEmail, "the provider didn't share an email address")
		}

		if claims.EmailVerified.set && !claims.EmailVerified.value {
			return errors.PermissionDenied(errors.ErrSSOEmailUnverified, "the provider hasn't verified the email address "+email)
		}

		var users []models.User
		if err := tx.Unscoped().Where("LOWER(email) = LOWER(?)", email).Limit(1).Find(&users).Error; err != nil {
			return err
		}

		switch {
		case len(users) > 0 && users[0].DeletedAt.Valid:
			return errors.PermissionDenied(errors.ErrSSOAccountDeleted, "the account of "+email+" was deleted")
		case len(users) > 0:
			userID = users[0].ID
		default:
			created, err := createUser(ctx, tx, provider, email, claims)
			if err != nil {
				return err
			}

			userID = created.ID
		}

		return tx.Create(&models.SSOIdentity{
			ProviderID:  provider.ID,
			Subject:     subject,
			UserID:      userID,
			Email:       email,
			LastLoginAt: now,
		}).Error
	})
	if err != nil {
		return nil, apiError(err, "failed to sign in")
	}

	var user models.User
	if err := c.db.WithContext(ctx).Preload("Roles.Permissions").First(&user, "id = ?", userID).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load the user")
	}

	return &user, nil
}

// createUser creates the account of someone signing in for the first time, with
// the provider's default role. Without one, nobody new may sign in.
func createUser(ctx context.Context, tx *gorm.DB, provider *models.SSOProvider, email string, claims profile) (*models.User, error) {
	noAccount := errors.PermissionDenied(errors.ErrSSONoAccount, "there is no Parlance account for "+email+"; ask an admin to add one")

	if provider.DefaultRoleID == nil {
		return nil, noAccount
	}

	var roles []models.Role
	if err := tx.Where("id = ?", *provider.DefaultRoleID).Limit(1).Find(&roles).Error; err != nil {
		return nil, err
	}

	// The role was deleted since; creating accounts without a role would hide that.
	if len(roles) == 0 {
		return nil, noAccount
	}

	return common.CreateUser(ctx, tx, common.NewUser{
		Email:    email,
		Name:     claims.displayName(),
		Roles:    roles,
		External: true,
	})
}

// requireActive refuses users who were deleted.
func requireActive(tx *gorm.DB, userID string) error {
	var users []models.User
	if err := tx.Unscoped().Where("id = ?", userID).Limit(1).Find(&users).Error; err != nil {
		return err
	}

	if len(users) == 0 || users[0].DeletedAt.Valid {
		return errors.PermissionDenied(errors.ErrSSOAccountDeleted, "the account was deleted")
	}

	return nil
}

// enabledProvider loads a provider that may be signed in with.
func (c *Client) enabledProvider(ctx context.Context, id string) (models.SSOProvider, error) {
	var providers []models.SSOProvider
	if err := c.db.WithContext(ctx).Where("id = ? AND enabled = ?", id, true).Limit(1).Find(&providers).Error; err != nil {
		return models.SSOProvider{}, errors.InternalCause(err, "failed to load the provider")
	}

	if len(providers) == 0 {
		return models.SSOProvider{}, errors.NotFound(errors.ErrSSOProviderNotFound, "the sign-in provider doesn't exist or is turned off")
	}

	return providers[0], nil
}

// takeLogin loads the sign-in the state belongs to and removes it, so its code can
// be used once.
func (c *Client) takeLogin(ctx context.Context, state string) (models.SSOLogin, error) {
	expired := errors.FailedPrecondition(errors.ErrSSOLoginExpired, "the sign-in expired or was finished already; start again")

	var login models.SSOLogin

	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var logins []models.SSOLogin
		if err := tx.Where("state = ?", state).Limit(1).Find(&logins).Error; err != nil {
			return err
		}

		if len(logins) == 0 {
			return expired
		}

		login = logins[0]

		// Removed in any case, as a second try must start over.
		result := tx.Where("state = ?", state).Delete(&models.SSOLogin{})
		if result.Error != nil {
			return result.Error
		}

		// Another request took it meanwhile.
		if result.RowsAffected == 0 || login.ExpiresAt.Before(c.now()) {
			return expired
		}

		return nil
	})
	if err != nil {
		return login, apiError(err, "failed to finish the sign-in")
	}

	return login, nil
}

// connect returns the OAuth settings of a sign-in with the provider, and its
// discovered configuration.
func (c *Client) connect(provider models.SSOProvider, redirectURI string) (*oauth2.Config, *oidc.Provider, error) {
	discovered, err := c.discover(provider.IssuerURL)
	if err != nil {
		return nil, nil, failed("the provider can't be reached", err)
	}

	clientSecret := ""

	if provider.ClientSecret != "" {
		if clientSecret, err = c.box.Decrypt(provider.ClientSecret); err != nil {
			return nil, nil, errors.FailedPrecondition(errors.ErrSSOFailed, "the provider's client secret can't be read, e.g. because ENCRYPTION_KEY changed; an admin must enter it again")
		}
	}

	return &oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: clientSecret,
		Endpoint:     discovered.Endpoint(),
		RedirectURL:  redirectURI,
		Scopes:       scopes,
	}, discovered, nil
}

// discover returns the issuer's configuration, fetched at most once an hour. Its
// signing keys are fetched later, as tokens need them, with the client's timeout.
func (c *Client) discover(issuerURL string) (*oidc.Provider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cached, ok := c.providers[issuerURL]; ok && c.now().Sub(cached.fetchedAt) < discoveryTTL {
		return cached.provider, nil
	}

	// Not the request's context: the provider keeps it to fetch keys later.
	provider, err := oidc.NewProvider(oidc.ClientContext(context.Background(), c.http), issuerURL)
	if err != nil {
		return nil, err
	}

	c.providers[issuerURL] = cachedProvider{provider: provider, fetchedAt: c.now()}

	return provider, nil
}

// failed is a sign-in the provider or its token made fail, with the cause to see
// what went wrong.
func failed(message string, cause error) error {
	if cause != nil {
		message = fmt.Sprintf("%s: %v", message, cause)
	}

	return errors.Unauthenticated(errors.ErrSSOFailed, message)
}

// apiError passes API errors on and hides others as internal ones.
func apiError(err error, message string) error {
	if connect.CodeOf(err) != connect.CodeUnknown {
		return err
	}

	return errors.InternalCause(err, message)
}

// randomString returns 128 random bits as text, for states and nonces.
func randomString() string {
	return rand.Text()
}
