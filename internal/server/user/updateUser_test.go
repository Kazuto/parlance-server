package user

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"google.golang.org/protobuf/proto"
)

// A field that is sent but empty must be rejected for required fields, before any database access.
func TestUpdateUserRejectsEmptyRequiredFields(t *testing.T) {
	server := &Server{}
	for _, msg := range []*pb.UpdateUserRequest{
		{Id: "user", Email: proto.String("")},
		{Id: "user", Name: proto.String("")},
		{Id: "user", Password: proto.String("")},
		{Id: "user", Avatar: []byte("png")},
	} {
		_, err := server.UpdateUser(context.Background(), connect.NewRequest(msg))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%v: expected invalid argument, got %v", msg, err)
		}
	}
}
