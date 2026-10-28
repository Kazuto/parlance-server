package account

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/apitoken"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

// viaToken makes the request as if made with an API token
func viaToken(ctx context.Context) context.Context {
	return context.WithValue(ctx, middleware.ContextKeyAPIToken, "token-id")
}

func TestAPITokens(t *testing.T) {
	ctx, server, _, user := setup(t)
	t.Cleanup(func() { server.db.Exec(`DELETE FROM api_tokens`) })

	create := func(ctx context.Context, req *pb.CreateApiTokenRequest) *pb.CreateApiTokenResponse {
		t.Helper()

		res, err := server.CreateApiToken(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("CreateApiToken(%v) error = %v", req, err)
		}

		return res.Msg
	}
	list := func() []*pb.ApiToken {
		t.Helper()

		res, err := server.ListApiTokens(ctx, connect.NewRequest(&pb.ListApiTokensRequest{}))
		if err != nil {
			t.Fatalf("ListApiTokens() error = %v", err)
		}

		return res.Msg.Tokens
	}

	ci := create(ctx, &pb.CreateApiTokenRequest{Name: " CI ", ExpiresInDays: 30})

	if !apitoken.IsToken(ci.Secret) || ci.Token.Prefix != ci.Secret[:10] || ci.Token.Name != "CI" {
		t.Errorf("created %+v with %q", ci.Token, ci.Secret)
	}

	if expires, err := time.Parse(time.RFC3339, ci.Token.GetExpiresAt()); err != nil ||
		time.Until(expires) < 29*24*time.Hour || time.Until(expires) > 30*24*time.Hour {
		t.Errorf("expires at %q, want in 30 days", ci.Token.GetExpiresAt())
	}

	// The token works, as its user.
	if _, userID, err := apitoken.NewAuthenticator(server.db.DB).Authenticate(ctx, ci.Secret); err != nil || userID != user.ID {
		t.Errorf("Authenticate() = %q, %v; want %q", userID, err, user.ID)
	}

	deploy := create(ctx, &pb.CreateApiTokenRequest{Name: "Deploy"})
	if deploy.Token.ExpiresAt != nil {
		t.Errorf("token without a lifetime expires at %q", deploy.Token.GetExpiresAt())
	}

	if tokens := list(); len(tokens) != 2 || tokens[0].Id != deploy.Token.Id || tokens[1].Id != ci.Token.Id {
		t.Errorf("listed %v, want Deploy, then CI", tokens)
	}

	testutil.ExpectInvalidArgument(t, ctx, server.CreateApiToken,
		&pb.CreateApiTokenRequest{Name: " "},
		&pb.CreateApiTokenRequest{Name: strings.Repeat("a", 101)},
		&pb.CreateApiTokenRequest{Name: "CI", ExpiresInDays: -1},
		&pb.CreateApiTokenRequest{Name: "CI", ExpiresInDays: 3651},
	)

	// Another user's token is not found, and stays.
	other, err := testutil.ContextWithPermissions(server.db)
	if err != nil {
		t.Fatal(err)
	}

	theirs := create(other, &pb.CreateApiTokenRequest{Name: "Theirs"})

	_, err = server.DeleteApiToken(ctx, connect.NewRequest(&pb.DeleteApiTokenRequest{Id: theirs.Token.Id}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("deleting another user's token: %v, want not found", err)
	}

	if _, err := server.DeleteApiToken(ctx, connect.NewRequest(&pb.DeleteApiTokenRequest{Id: ci.Token.Id})); err != nil {
		t.Fatalf("DeleteApiToken() error = %v", err)
	}

	if tokens := list(); len(tokens) != 1 || tokens[0].Id != deploy.Token.Id {
		t.Errorf("listed %v after revoking CI, want Deploy", tokens)
	}

	if _, _, err := apitoken.NewAuthenticator(server.db.DB).Authenticate(ctx, theirs.Secret); err != nil {
		t.Errorf("the other user's token: %v", err)
	}
}

func TestAPITokenLimit(t *testing.T) {
	ctx, server, _, user := setup(t)
	t.Cleanup(func() { server.db.Exec(`DELETE FROM api_tokens`) })

	for i := range maxAPITokens {
		token := models.APIToken{UserID: user.ID, Name: "CI", TokenHash: apitoken.Hash(string(rune('a' + i))), Prefix: "plc_"}
		if err := server.db.Create(&token).Error; err != nil {
			t.Fatal(err)
		}
	}

	_, err := server.CreateApiToken(ctx, connect.NewRequest(&pb.CreateApiTokenRequest{Name: "One more"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), errors.ErrAPITokenLimit) {
		t.Errorf("creating token %d: %v, want the limit", maxAPITokens+1, err)
	}
}

func TestAPITokensCannotTakeTheAccountOver(t *testing.T) {
	ctx, server, _, user := setup(t)
	token := viaToken(ctx)

	calls := map[string]func() error{
		"list tokens": func() error {
			_, err := server.ListApiTokens(token, connect.NewRequest(&pb.ListApiTokensRequest{}))

			return err
		},
		"create a token": func() error {
			_, err := server.CreateApiToken(token, connect.NewRequest(&pb.CreateApiTokenRequest{Name: "More"}))

			return err
		},
		"revoke a token": func() error {
			_, err := server.DeleteApiToken(token, connect.NewRequest(&pb.DeleteApiTokenRequest{Id: "any"}))

			return err
		},
		"change the password": func() error {
			_, err := server.ChangePassword(token, connect.NewRequest(&pb.ChangePasswordRequest{
				CurrentPassword: "old-password", NewPassword: "new-password",
			}))

			return err
		},
		"change the email": func() error {
			_, err := server.UpdateAccount(token, connect.NewRequest(&pb.UpdateAccountRequest{Email: proto.String("taken-over@example.com")}))

			return err
		},
	}

	for name, call := range calls {
		if err := call(); connect.CodeOf(err) != connect.CodePermissionDenied || !strings.Contains(err.Error(), errors.ErrAPITokenNotAllowed) {
			t.Errorf("%s with an API token: %v, want it denied", name, err)
		}
	}

	// The rest of the account may change, the same email included.
	_, err := server.UpdateAccount(token, connect.NewRequest(&pb.UpdateAccountRequest{Name: proto.String("Renamed"), Email: proto.String(user.Email)}))
	if err != nil {
		t.Errorf("renaming with an API token: %v", err)
	}
}
