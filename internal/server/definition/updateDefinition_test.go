package definition

import (
	"testing"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestUpdateDefinitionRejectsEmptyRequiredFields(t *testing.T) {
	ctx, db := testutil.Setup(t)
	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db}).UpdateDefinition,
		&pb.UpdateDefinitionRequest{Id: "definition", Translation: proto.String("")},
	)
}
