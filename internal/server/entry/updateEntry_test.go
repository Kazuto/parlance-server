package entry

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"google.golang.org/protobuf/proto"
)

func TestUpdateEntryRejectsEmptyKey(t *testing.T) {
	server := &Server{}
	for _, key := range []string{"", "   "} {
		_, err := server.UpdateEntry(context.Background(), connect.NewRequest(&pb.UpdateEntryRequest{Id: "entry", Key: proto.String(key)}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("expected invalid argument, got %v", err)
		}
	}
}
