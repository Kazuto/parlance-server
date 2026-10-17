package definition

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

// TestUpdateDefinitionRejectsEmptyRequiredFields verifies that empty required fields
// are rejected with proper validation errors
func TestUpdateDefinitionRejectsEmptyRequiredFields(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}

	server := &Server{db: db}

	for _, msg := range []*pb.UpdateDefinitionRequest{
		{Id: "definition", Translation: proto.String("")},
	} {
		_, err := server.UpdateDefinition(ctx, connect.NewRequest(msg))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%v: expected invalid argument, got %v", msg, err)
		}
	}
}
