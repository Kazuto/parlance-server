package importservice

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestReplaceAllImportRequiresDeletePermission(t *testing.T) {
	_, db := testutil.Setup(t)

	ctx, err := testutil.ContextWithPermissions(db, "create_entry", "update_entry")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	server := &Server{db: db}

	_, err = server.ImportJson(ctx, connect.NewRequest(&pb.ImportJsonRequest{Mode: pb.ImportMode_IMPORT_MODE_REPLACE_ALL}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}

	_, err = server.ImportJson(ctx, connect.NewRequest(&pb.ImportJsonRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected create/update import to pass the permission check, got %v", err)
	}
}
