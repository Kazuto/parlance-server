package scope

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestUpdateScopeRejectsEmptyRequiredFields(t *testing.T) {
	ctx, db := testutil.Setup(t)
	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db}).UpdateScope,
		&pb.UpdateScopeRequest{Id: "scope", Name: proto.String("")},
		&pb.UpdateScopeRequest{Id: "scope", Slug: proto.String("")},
	)
}

func TestScopeExportFormat(t *testing.T) {
	ctx, db := testutil.Setup(t)

	server := &Server{db: db}

	testutil.ExpectInvalidArgument(t, ctx, server.CreateScope,
		&pb.CreateScopeRequest{Name: "Mail", Slug: "mail", ExportFormat: pb.ExportFormat(99)},
	)

	created, err := server.CreateScope(ctx, connect.NewRequest(&pb.CreateScopeRequest{
		Name: "Mail", Slug: "mail", ExportFormat: pb.ExportFormat_EXPORT_FORMAT_PHP,
	}))
	if err != nil {
		t.Fatalf("CreateScope: %v", err)
	}

	if created.Msg.Scope.ExportFormat != pb.ExportFormat_EXPORT_FORMAT_PHP {
		t.Errorf("created with %v, want php", created.Msg.Scope.ExportFormat)
	}

	id := created.Msg.Scope.Id

	testutil.ExpectInvalidArgument(t, ctx, server.UpdateScope,
		&pb.UpdateScopeRequest{Id: id, ExportFormat: pb.ExportFormat(99).Enum()},
	)

	steps := []struct {
		name string
		req  *pb.UpdateScopeRequest
		want string
	}{
		{"change", &pb.UpdateScopeRequest{Id: id, ExportFormat: pb.ExportFormat_EXPORT_FORMAT_PO.Enum()}, "po"},
		{"leave out", &pb.UpdateScopeRequest{Id: id, Name: proto.String("Mailing")}, "po"},
		{"clear", &pb.UpdateScopeRequest{Id: id, ExportFormat: pb.ExportFormat_EXPORT_FORMAT_UNSPECIFIED.Enum()}, ""},
	}

	for _, step := range steps {
		if _, err := server.UpdateScope(ctx, connect.NewRequest(step.req)); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}

		var format string

		db.Raw("SELECT export_format FROM scopes WHERE id = ?", id).Scan(&format)

		if format != step.want {
			t.Errorf("%s: stored %q, want %q", step.name, format, step.want)
		}
	}
}
