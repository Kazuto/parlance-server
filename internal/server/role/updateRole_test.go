package role

import (
	"testing"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestUpdateRoleRejectsEmptyRequiredFields(t *testing.T) {
	ctx, db := testutil.Setup(t)
	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db}).UpdateRole,
		&pb.UpdateRoleRequest{Id: "role", Name: proto.String("")},
	)
}
