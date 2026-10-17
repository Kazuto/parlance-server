package user

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestUpdateUserRejectsEmptyRequiredFields(t *testing.T) {
	ctx, db, err := testutil.SetupTestContext()
	if err != nil {
		t.Fatalf("Failed to setup test context: %v", err)
	}

	server := &Server{db: db}
	for _, msg := range []*pb.UpdateUserRequest{
		{Id: "user", Email: proto.String("")},
		{Id: "user", Name: proto.String("")},
		{Id: "user", Password: proto.String("")},
		{Id: "user", Avatar: []byte("png")},
	} {
		_, err := server.UpdateUser(ctx, connect.NewRequest(msg))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%v: expected invalid argument, got %v", msg, err)
		}
	}
}
