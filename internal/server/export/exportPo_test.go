package export

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func TestExportPoRequiresLocale(t *testing.T) {
	server := &Server{}
	if _, err := server.ExportPo(context.Background(), connect.NewRequest(&pb.ExportPoRequest{})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}
