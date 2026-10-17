package entry

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestUpdateEntryRejectsEmptyKey(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}

	server := &Server{db: db}

	for _, key := range []string{"", "   "} {
		_, err := server.UpdateEntry(ctx, connect.NewRequest(&pb.UpdateEntryRequest{Id: "entry", Key: proto.String(key)}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("expected invalid argument, got %v", err)
		}
	}
}
