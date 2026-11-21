package auth

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/version"
)

func TestGetCurrentUserTellsTheServerVersion(t *testing.T) {
	ctx, db := testutil.Setup(t)

	previous := version.Version
	version.Version = "2.1.0"

	t.Cleanup(func() { version.Version = previous })

	res, err := NewServer(db, &config.Config{}, nil).GetCurrentUser(ctx, connect.NewRequest(&pb.GetCurrentUserRequest{}))
	if err != nil {
		t.Fatalf("GetCurrentUser() error = %v", err)
	}

	if res.Msg.ServerVersion != "2.1.0" {
		t.Errorf("ServerVersion = %q, want 2.1.0", res.Msg.ServerVersion)
	}
}
