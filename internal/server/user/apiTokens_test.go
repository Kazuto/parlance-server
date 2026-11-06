package user

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/apitoken"
	"github.com/kazuto/parlance-server/internal/avatar"
	"github.com/kazuto/parlance-server/internal/avatar/avatartest"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestAPITokenCannotChangeItsOwnSignIn(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db, avatars: avatar.New(avatartest.NewMemory())}
	token := context.WithValue(ctx, middleware.ContextKeyAPIToken, "token-id")
	self := middleware.UserID(ctx)

	other, err := testutil.ContextWithPermissions(db)
	if err != nil {
		t.Fatal(err)
	}

	update := func(req *pb.UpdateUserRequest) error {
		_, err := server.UpdateUser(token, connect.NewRequest(req))

		return err
	}

	for name, req := range map[string]*pb.UpdateUserRequest{
		"own password": {Id: self, Password: proto.String("taken-over")},
		"own email":    {Id: self, Email: proto.String("taken-over@example.com")},
	} {
		if err := update(req); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s with an API token: %v, want it denied", name, err)
		}
	}

	// Other users, and the rest of one's own, are up to the token's permissions.
	for name, req := range map[string]*pb.UpdateUserRequest{
		"another user's password": {Id: middleware.UserID(other), Password: proto.String("reset-by-admin")},
		"own name":                {Id: self, Name: proto.String("Renamed")},
	} {
		if err := update(req); err != nil {
			t.Errorf("%s with an API token: %v", name, err)
		}
	}
}

func TestDeleteUserRevokesTheirAPITokens(t *testing.T) {
	ctx, db := testutil.Setup(t)
	t.Cleanup(func() { db.Exec(`DELETE FROM api_tokens`) })

	other, err := testutil.ContextWithPermissions(db)
	if err != nil {
		t.Fatal(err)
	}

	userID := middleware.UserID(other)
	token := models.APIToken{UserID: userID, Name: "CI", TokenHash: apitoken.Hash("plc_x"), Prefix: "plc_x"}

	if err := db.Create(&token).Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{db: db, avatars: avatar.New(avatartest.NewMemory())}
	if _, err := server.DeleteUser(ctx, connect.NewRequest(&pb.DeleteUserRequest{Id: userID})); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	var count int64
	if db.Model(&models.APIToken{}).Where("user_id = ?", userID).Count(&count); count != 0 {
		t.Errorf("%d API tokens left, want none", count)
	}
}

func TestRegistrationSettings(t *testing.T) {
	ctx, db := testutil.Setup(t)
	t.Cleanup(func() { db.Exec(`DELETE FROM settings`) })

	server := &Server{db: db}

	got, err := server.GetRegistrationSettings(ctx, connect.NewRequest(&pb.GetRegistrationSettingsRequest{}))
	if err != nil || !got.Msg.Settings.Open || got.Msg.Settings.UpdatedAt != nil {
		t.Fatalf("settings before any change = %v, %v; want open", got, err)
	}

	updated, err := server.UpdateRegistrationSettings(ctx, connect.NewRequest(&pb.UpdateRegistrationSettingsRequest{Open: false}))
	if err != nil || updated.Msg.Settings.Open || updated.Msg.Settings.UpdatedAt == nil {
		t.Fatalf("closing = %v, %v", updated, err)
	}

	var setting models.Setting
	if err := db.First(&setting, "key = ?", models.SettingRegistrationOpen).Error; err != nil ||
		setting.UpdatedBy == nil || *setting.UpdatedBy != middleware.UserID(ctx) {
		t.Errorf("saved %+v, %v; want it credited to the admin", setting, err)
	}
}
