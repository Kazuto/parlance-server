package export

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestExportPoRequiresLocale(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}

	if _, err := server.ExportPo(ctx, connect.NewRequest(&pb.ExportPoRequest{})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}
