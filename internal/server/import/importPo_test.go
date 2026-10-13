package importservice

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func TestImportPoInvalidInput(t *testing.T) {
	server := &Server{}
	for _, request := range []*pb.ImportPoRequest{
		{},
		{LocaleId: "locale"},
		{LocaleId: "locale", FileContent: []byte("msgid \"broken\"\n")},
		{LocaleId: "locale", Mode: pb.ImportMode_IMPORT_MODE_REPLACE_ALL, FileContent: []byte("msgctxt \"menu\"\nmsgid \"save\"\nmsgstr \"Save\"\n")},
	} {
		if _, err := server.ImportPo(context.Background(), connect.NewRequest(request)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("request %+v: expected invalid argument, got %v", request, err)
		}
	}
}
