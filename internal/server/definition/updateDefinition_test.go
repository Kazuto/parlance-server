package definition

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"google.golang.org/protobuf/proto"
)

// A field that is sent but empty must be rejected for required fields, before any database access.
func TestUpdateDefinitionRejectsEmptyRequiredFields(t *testing.T) {
	server := &Server{}
	for _, msg := range []*pb.UpdateDefinitionRequest{
		{Id: "definition", Translation: proto.String("")},
	} {
		_, err := server.UpdateDefinition(context.Background(), connect.NewRequest(msg))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%v: expected invalid argument, got %v", msg, err)
		}
	}
}
