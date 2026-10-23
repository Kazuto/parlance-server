package locale

import (
	"testing"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestUpdateLocaleRejectsEmptyRequiredFields(t *testing.T) {
	ctx, db := testutil.Setup(t)
	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db}).UpdateLocale,
		&pb.UpdateLocaleRequest{Id: "locale", Code: proto.String("")},
		&pb.UpdateLocaleRequest{Id: "locale", Names: &pb.StringMap{}},
	)
}
