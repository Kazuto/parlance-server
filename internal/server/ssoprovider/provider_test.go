package ssoprovider

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func setup(t *testing.T) (context.Context, *Server, *[]string) {
	t.Helper()

	ctx, db := testutil.Setup(t)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM sso_logins`)
		db.Exec(`DELETE FROM sso_identities`)
		db.Exec(`DELETE FROM sso_providers`)
	})

	box, _ := secret.New("test key")
	checked := []string{}

	server := NewServer(db, box, func(_ context.Context, issuer string) error {
		checked = append(checked, issuer)
		if issuer == "https://down.example" {
			return errors.New("connection refused")
		}

		return nil
	}).(*Server)

	return ctx, server, &checked
}

func adminRoleID(t *testing.T, server *Server) string {
	t.Helper()

	var role models.Role
	if err := server.db.First(&role, "name = ?", "Test Admin").Error; err != nil {
		t.Fatal(err)
	}

	return role.ID
}

func TestCreateSsoProvider(t *testing.T) {
	ctx, server, _ := setup(t)
	roleID := adminRoleID(t, server)

	res, err := server.CreateSsoProvider(ctx, connect.NewRequest(&pb.CreateSsoProviderRequest{
		Name: " Authentik ", IssuerUrl: " https://auth.example/application/o/parlance/ ", ClientId: " parlance ",
		ClientSecret: "s3cret", DefaultRoleId: &roleID, Enabled: true,
	}))
	if err != nil {
		t.Fatalf("CreateSsoProvider() error = %v", err)
	}

	provider := res.Msg.SsoProvider
	// The issuer keeps its trailing slash: the provider must name itself the same way.
	if provider.Name != "Authentik" || provider.IssuerUrl != "https://auth.example/application/o/parlance/" ||
		provider.ClientId != "parlance" || !provider.HasClientSecret || provider.GetDefaultRoleId() != roleID || !provider.Enabled {
		t.Errorf("provider = %+v", provider)
	}

	var stored models.SSOProvider

	server.db.First(&stored, "id = ?", provider.Id)

	if plain, err := server.box.Decrypt(stored.ClientSecret); err != nil || plain != "s3cret" {
		t.Errorf("stored secret = %q, %v", plain, err)
	}

	missing := "missing"

	testutil.ExpectInvalidArgument(t, ctx, server.CreateSsoProvider,
		&pb.CreateSsoProviderRequest{IssuerUrl: "https://auth.example", ClientId: "parlance"},
		&pb.CreateSsoProviderRequest{Name: "No issuer", ClientId: "parlance"},
		&pb.CreateSsoProviderRequest{Name: "Relative", IssuerUrl: "/realms/main", ClientId: "parlance"},
		&pb.CreateSsoProviderRequest{Name: "No client", IssuerUrl: "https://auth.example"},
	)

	if _, err := server.CreateSsoProvider(ctx, connect.NewRequest(&pb.CreateSsoProviderRequest{
		Name: "Bad role", IssuerUrl: "https://auth.example", ClientId: "parlance", DefaultRoleId: &missing,
	})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown role: error = %v, want not found", err)
	}
}

func TestUpdateSsoProvider(t *testing.T) {
	ctx, server, _ := setup(t)
	roleID := adminRoleID(t, server)

	created, err := server.CreateSsoProvider(ctx, connect.NewRequest(&pb.CreateSsoProviderRequest{
		Name: "Keycloak", IssuerUrl: "https://auth.example/realms/main", ClientId: "parlance", ClientSecret: "first",
		DefaultRoleId: &roleID, Enabled: true,
	}))
	if err != nil {
		t.Fatal(err)
	}

	id := created.Msg.SsoProvider.Id

	res, err := server.UpdateSsoProvider(ctx, connect.NewRequest(&pb.UpdateSsoProviderRequest{Id: id, Name: proto.String("Company login")}))
	if err != nil || res.Msg.SsoProvider.Name != "Company login" || !res.Msg.SsoProvider.HasClientSecret || res.Msg.SsoProvider.GetDefaultRoleId() != roleID {
		t.Errorf("renamed = %+v, %v; want the rest kept", res, err)
	}

	// Empty values remove the secret and stop creating accounts.
	res, err = server.UpdateSsoProvider(ctx, connect.NewRequest(&pb.UpdateSsoProviderRequest{
		Id: id, ClientSecret: proto.String(""), DefaultRoleId: proto.String(""), Enabled: proto.Bool(false),
	}))
	if err != nil || res.Msg.SsoProvider.HasClientSecret || res.Msg.SsoProvider.DefaultRoleId != nil || res.Msg.SsoProvider.Enabled {
		t.Errorf("cleared = %+v, %v", res, err)
	}
}

func TestDeleteAndTestSsoProvider(t *testing.T) {
	ctx, server, checked := setup(t)

	up := models.SSOProvider{Name: "Up", IssuerURL: "https://auth.example", ClientID: "parlance"}
	down := models.SSOProvider{Name: "Down", IssuerURL: "https://down.example", ClientID: "parlance"}

	server.db.Create(&up)
	server.db.Create(&down)

	ok, err := server.TestSsoProvider(ctx, connect.NewRequest(&pb.TestSsoProviderRequest{Id: up.ID}))
	if err != nil || !ok.Msg.Ok {
		t.Errorf("test = %+v, %v", ok, err)
	}

	failed, err := server.TestSsoProvider(ctx, connect.NewRequest(&pb.TestSsoProviderRequest{Id: down.ID}))
	if err != nil || failed.Msg.Ok || failed.Msg.Error != "connection refused" {
		t.Errorf("test = %+v, %v", failed, err)
	}

	if len(*checked) != 2 {
		t.Errorf("checked %v", *checked)
	}

	server.db.Create(&models.SSOIdentity{ProviderID: up.ID, Subject: "x", UserID: "user"})

	if _, err := server.DeleteSsoProvider(ctx, connect.NewRequest(&pb.DeleteSsoProviderRequest{Id: up.ID})); err != nil {
		t.Fatalf("DeleteSsoProvider() error = %v", err)
	}

	var left int64

	server.db.Model(&models.SSOIdentity{}).Where("provider_id = ?", up.ID).Count(&left)

	if left != 0 {
		t.Errorf("%d links left", left)
	}
}
