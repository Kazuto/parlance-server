package entry

import (
	"testing"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestUpdateEntryRejectsEmptyKey(t *testing.T) {
	ctx, db := testutil.Setup(t)
	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db}).UpdateEntry,
		&pb.UpdateEntryRequest{Id: "entry", Key: proto.String("")},
		&pb.UpdateEntryRequest{Id: "entry", Key: proto.String("   ")},
	)
}
