package user

import (
	"testing"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestUpdateUserRejectsEmptyRequiredFields(t *testing.T) {
	ctx, db := testutil.Setup(t)
	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db}).UpdateUser,
		&pb.UpdateUserRequest{Id: "user", Email: proto.String("")},
		&pb.UpdateUserRequest{Id: "user", Name: proto.String("")},
		&pb.UpdateUserRequest{Id: "user", Password: proto.String("")},
		&pb.UpdateUserRequest{Id: "user", Avatar: []byte("png")},
	)
}
