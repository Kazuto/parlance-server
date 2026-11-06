package user

import (
	"testing"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/avatar"
	"github.com/kazuto/parlance-server/internal/avatar/avatartest"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestUpdateUserRejectsEmptyRequiredFields(t *testing.T) {
	ctx, db := testutil.Setup(t)
	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db, avatars: avatar.New(avatartest.NewMemory())}).UpdateUser,
		&pb.UpdateUserRequest{Id: "user", Email: proto.String("")},
		&pb.UpdateUserRequest{Id: "user", Name: proto.String("")},
		&pb.UpdateUserRequest{Id: "user", Password: proto.String("")},
		// The image's type is read from it, whatever the client says.
		&pb.UpdateUserRequest{Id: middleware.UserID(ctx), Avatar: []byte("<html></html>"), AvatarMime: proto.String("image/png")},
	)
}
