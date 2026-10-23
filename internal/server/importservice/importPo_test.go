package importservice

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestImportPoInvalidInput(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db} // Invalid input must fail before the database changes.

	for _, request := range []*pb.ImportPoRequest{
		{},
		{LocaleId: "locale"},
		{LocaleId: "locale", FileContent: []byte("msgid \"broken\"\n")},
		{LocaleId: "locale", Mode: pb.ImportMode_IMPORT_MODE_REPLACE_ALL, FileContent: []byte("msgctxt \"menu\"\nmsgid \"save\"\nmsgstr \"Save\"\n")},
	} {
		if _, err := server.ImportPo(ctx, connect.NewRequest(request)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("request %+v: expected invalid argument, got %v", request, err)
		}
	}
}
