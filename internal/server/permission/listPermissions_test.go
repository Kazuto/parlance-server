package permission

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestListPermissionsSearchIgnoresCase(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}

	res, err := server.ListPermissions(ctx, connect.NewRequest(&pb.ListPermissionsRequest{Filter: &pb.FilterRequest{Search: "Translation_Server"}}))
	if err != nil {
		t.Fatalf("ListPermissions() error = %v", err)
	}

	if len(res.Msg.Permissions) != 4 || res.Msg.Pagination.Total != 4 {
		t.Errorf("got %d permissions (total %d), want the 4 translation server permissions", len(res.Msg.Permissions), res.Msg.Pagination.Total)
	}
}
