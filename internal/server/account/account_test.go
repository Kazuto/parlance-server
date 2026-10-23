package account

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/avatar"
	"github.com/kazuto/parlance-server/internal/avatar/avatartest"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/rbac"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

var png = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), make([]byte, 64)...)

// setup signs in a user without any permission.
func setup(t *testing.T) (context.Context, *Server, *avatartest.Memory, models.User) {
	t.Helper()

	_, db := testutil.Setup(t)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM users WHERE email <> 'test@example.com'`)
	})

	ctx, err := testutil.ContextWithPermissions(db)
	if err != nil {
		t.Fatal(err)
	}

	var user models.User
	if err := db.Where("name = ?", "Limited User").Last(&user).Error; err != nil {
		t.Fatal(err)
	}

	hash, _ := auth.HashPassword("old-password")
	db.Model(&user).Update("password_hash", hash)

	storage := avatartest.NewMemory()

	return ctx, NewServer(db, avatar.New(storage)).(*Server), storage, user
}

func TestAnyoneSignedInMayManageTheirAccount(t *testing.T) {
	ctx, server, _, _ := setup(t)

	for _, procedure := range []string{
		parlancev1connect.AccountServiceUpdateAccountProcedure,
		parlancev1connect.AccountServiceChangePasswordProcedure,
	} {
		if _, err := rbac.Authorize(ctx, server.db, procedure); err != nil {
			t.Errorf("%s: %v, want it allowed without permissions", procedure, err)
		}
	}
}

func TestUpdateAccount(t *testing.T) {
	ctx, server, storage, user := setup(t)

	res, err := server.UpdateAccount(ctx, connect.NewRequest(&pb.UpdateAccountRequest{
		Name: proto.String(" Ada "), Email: proto.String(" ada@example.com "), Locale: proto.String("de"), Avatar: png,
	}))
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}

	updated := res.Msg.User
	if updated.Id != user.ID || updated.Name != "Ada" || updated.Email != "ada@example.com" ||
		!strings.HasPrefix(updated.Avatar, "/avatars/"+user.ID+"?v=") || !updated.HasPassword {
		t.Errorf("user = %+v", updated)
	}

	var stored models.User

	server.db.First(&stored, "id = ?", user.ID)

	if stored.Locale == nil || *stored.Locale != "de" || !storage.Has("avatars/"+user.ID) {
		t.Errorf("stored = %+v, avatar stored = %v", stored, storage.Has("avatars/"+user.ID))
	}

	// An empty image removes the avatar.
	res, err = server.UpdateAccount(ctx, connect.NewRequest(&pb.UpdateAccountRequest{Avatar: []byte{}}))
	if err != nil || res.Msg.User.Avatar != "" || storage.Has("avatars/"+user.ID) || res.Msg.User.Name != "Ada" {
		t.Errorf("after removing the avatar: %+v, %v", res, err)
	}
}

func TestUpdateAccountRejections(t *testing.T) {
	ctx, server, _, _ := setup(t)

	// testutil's admin has this address, in another case.
	if _, err := server.UpdateAccount(ctx, connect.NewRequest(&pb.UpdateAccountRequest{Email: proto.String("TEST@example.com")})); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("taken email: error = %v, want already exists", err)
	}

	testutil.ExpectInvalidArgument(t, ctx, server.UpdateAccount,
		&pb.UpdateAccountRequest{Name: proto.String(" ")},
		&pb.UpdateAccountRequest{Email: proto.String("not an address")},
		&pb.UpdateAccountRequest{Avatar: []byte("<html></html>")},
	)

	if _, err := server.UpdateAccount(context.Background(), connect.NewRequest(&pb.UpdateAccountRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("signed out: error = %v, want unauthenticated", err)
	}
}

func TestChangePassword(t *testing.T) {
	ctx, server, _, user := setup(t)
	change := func(current, next string) error {
		_, err := server.ChangePassword(ctx, connect.NewRequest(&pb.ChangePasswordRequest{CurrentPassword: current, NewPassword: next}))

		return err
	}
	stored := func() string {
		var row models.User

		server.db.First(&row, "id = ?", user.ID)

		return row.PasswordHash
	}

	if err := change("wrong", "new-password"); err == nil || !strings.Contains(err.Error(), "[WRONG_PASSWORD]") {
		t.Errorf("wrong current password: error = %v", err)
	}

	if err := change("old-password", ""); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("empty new password: error = %v", err)
	}

	if err := change("old-password", "new-password"); err != nil || auth.CheckPassword("new-password", stored()) != nil {
		t.Errorf("change: error = %v", err)
	}

	// Accounts single sign-on created have no password to confirm.
	server.db.Model(&user).Update("password_hash", "")

	if err := change("", "first-password"); err != nil || auth.CheckPassword("first-password", stored()) != nil {
		t.Errorf("first password: error = %v", err)
	}
}
