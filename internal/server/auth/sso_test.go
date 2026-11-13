package auth

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	authpkg "github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/sso"
	"github.com/kazuto/parlance-server/internal/sso/ssotest"
	"github.com/kazuto/parlance-server/internal/webhook"
)

func TestSignInWithSso(t *testing.T) {
	_, db := testutil.Setup(t)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM sso_logins`)
		db.Exec(`DELETE FROM sso_identities`)
		db.Exec(`DELETE FROM sso_providers`)
	})

	idp := ssotest.New(t)
	idp.Account = map[string]any{"sub": "test-1", "email": "test@example.com", "email_verified": true}

	box, _ := secret.New("test key")
	encrypted, _ := box.Encrypt(idp.ClientSecret)

	enabled := models.SSOProvider{Name: "Keycloak", IssuerURL: idp.Issuer(), ClientID: idp.ClientID, ClientSecret: encrypted, Enabled: true}
	disabled := models.SSOProvider{Name: "Old", IssuerURL: idp.Issuer(), ClientID: "old"}

	db.Create(&enabled)
	db.Create(&disabled)

	cfg := &config.Config{
		JWT:  config.JWTConfig{Secret: "jwt-secret", Expiry: "1h"},
		CORS: config.CORSConfig{AllowedOrigins: []string{"http://app.test"}},
	}
	server := NewServer(db, cfg, sso.New(db.DB, box, nil))
	ctx := context.Background()

	listed, err := server.ListSignInProviders(ctx, connect.NewRequest(&pb.ListSignInProvidersRequest{}))
	if err != nil || len(listed.Msg.Providers) != 1 || listed.Msg.Providers[0].Name != "Keycloak" {
		t.Fatalf("providers = %+v, %v; want only the enabled one", listed, err)
	}

	testutil.ExpectInvalidArgument(t, ctx, server.StartSso,
		&pb.StartSsoRequest{RedirectUri: "http://app.test/login/sso"},
		&pb.StartSsoRequest{ProviderId: enabled.ID, RedirectUri: "/login/sso"},
		&pb.StartSsoRequest{ProviderId: enabled.ID, RedirectUri: "https://evil.example/login/sso"},
	)

	started, err := server.StartSso(ctx, connect.NewRequest(&pb.StartSsoRequest{ProviderId: enabled.ID, RedirectUri: "http://app.test/login/sso"}))
	if err != nil {
		t.Fatalf("StartSso() error = %v", err)
	}

	state, code := idp.SignIn(t, started.Msg.AuthorizationUrl)

	finished, err := server.FinishSso(ctx, connect.NewRequest(&pb.FinishSsoRequest{State: state, Code: code}))
	if err != nil {
		t.Fatalf("FinishSso() error = %v", err)
	}

	claims, err := authpkg.ValidateAccessToken(finished.Msg.AccessToken, "jwt-secret")
	if err != nil || claims.UserID != finished.Msg.User.Id || finished.Msg.User.Email != "test@example.com" {
		t.Errorf("signed in as %+v (%v), want testutil's user with a valid token", finished.Msg.User, err)
	}

	if _, err := authpkg.ValidateRefreshToken(finished.Msg.RefreshToken, "jwt-secret"); err != nil {
		t.Errorf("refresh token: %v", err)
	}
}

func TestFailedSsoSignInsAreReported(t *testing.T) {
	_, db := testutil.Setup(t)
	sent := loginHook(t, db)

	t.Cleanup(func() {
		db.Exec(`DELETE FROM sso_logins`)
		db.Exec(`DELETE FROM sso_identities`)
		db.Exec(`DELETE FROM sso_providers`)
	})

	// Without a default role, nobody new may sign in.
	idp := ssotest.New(t)
	idp.Account = map[string]any{"sub": "stranger-1", "email": "stranger@example.com", "email_verified": true}

	box, _ := secret.New("test key")
	encrypted, _ := box.Encrypt(idp.ClientSecret)
	provider := models.SSOProvider{Name: "Keycloak", IssuerURL: idp.Issuer(), ClientID: idp.ClientID, ClientSecret: encrypted, Enabled: true}
	db.Create(&provider)

	cfg := &config.Config{
		JWT:  config.JWTConfig{Secret: "jwt-secret", Expiry: "1h"},
		CORS: config.CORSConfig{AllowedOrigins: []string{"http://app.test"}},
	}
	server := NewServer(db, cfg, sso.New(db.DB, box, nil))
	ctx := context.Background()

	started, err := server.StartSso(ctx, connect.NewRequest(&pb.StartSsoRequest{ProviderId: provider.ID, RedirectUri: "http://app.test/login/sso"}))
	if err != nil {
		t.Fatalf("StartSso() error = %v", err)
	}

	state, code := idp.SignIn(t, started.Msg.AuthorizationUrl)

	if _, err := server.FinishSso(ctx, connect.NewRequest(&pb.FinishSsoRequest{State: state, Code: code})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("FinishSso() error = %v, want no account", err)
	}

	// Finishing it again, e.g. on reload, is an expired sign-in, which isn't reported.
	if _, err := server.FinishSso(ctx, connect.NewRequest(&pb.FinishSsoRequest{State: state, Code: code})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("FinishSso() again error = %v, want expired", err)
	}

	got, _ := sent()
	want := webhook.LoginFailedData{
		Email:    "stranger@example.com",
		Method:   webhook.LoginMethodSSO,
		Provider: &webhook.ProviderInfo{ID: provider.ID, Name: "Keycloak"},
		Reason:   "sso_no_account",
		Attempts: 1,
	}

	if len(got) != 1 || got[0].Provider == nil || *got[0].Provider != *want.Provider {
		t.Fatalf("sent %+v, want one event about the provider", got)
	}

	got[0].Provider = want.Provider
	if got[0] != want {
		t.Errorf("sent %+v\nwant %+v", got[0], want)
	}
}
