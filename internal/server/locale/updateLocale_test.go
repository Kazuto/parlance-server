package locale

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/models"
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

func TestLocaleFormality(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}

	t.Cleanup(func() { db.Exec(`DELETE FROM locales WHERE code = 'de-AT'`) })

	created, err := server.CreateLocale(ctx, connect.NewRequest(&pb.CreateLocaleRequest{
		Code:      "de-AT",
		Names:     &pb.StringMap{Values: map[string]string{"en": "German (Austria)"}},
		Formality: pb.Formality_FORMALITY_FORMAL,
	}))
	if err != nil {
		t.Fatalf("CreateLocale() error = %v", err)
	}

	id := created.Msg.Locale.Id

	if got := created.Msg.Locale.Formality; got != pb.Formality_FORMALITY_FORMAL {
		t.Errorf("created with formality %v, want formal", got)
	}

	update := func(req *pb.UpdateLocaleRequest) pb.Formality {
		t.Helper()

		req.Id = id

		res, err := server.UpdateLocale(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateLocale(%v) error = %v", req, err)
		}

		var stored models.Locale
		if err := db.First(&stored, "id = ?", id).Error; err != nil {
			t.Fatal(err)
		}

		if want := converter.LocaleToProto(&stored, "").Formality; res.Msg.Locale.Formality != want {
			t.Errorf("answered %v, but stored %v", res.Msg.Locale.Formality, want)
		}

		return res.Msg.Locale.Formality
	}

	if got := update(&pb.UpdateLocaleRequest{Formality: pb.Formality_FORMALITY_INFORMAL.Enum()}); got != pb.Formality_FORMALITY_INFORMAL {
		t.Errorf("changed to %v, want informal", got)
	}

	// Other changes keep it.
	if got := update(&pb.UpdateLocaleRequest{IsDefault: proto.Bool(false)}); got != pb.Formality_FORMALITY_INFORMAL {
		t.Errorf("after another change: %v, want informal", got)
	}

	if got := update(&pb.UpdateLocaleRequest{Formality: pb.Formality_FORMALITY_UNSPECIFIED.Enum()}); got != pb.Formality_FORMALITY_UNSPECIFIED {
		t.Errorf("cleared to %v, want unspecified", got)
	}

	unknown := pb.Formality(7)

	testutil.ExpectInvalidArgument(t, ctx, server.UpdateLocale,
		&pb.UpdateLocaleRequest{Id: id, Formality: &unknown},
	)
	testutil.ExpectInvalidArgument(t, ctx, server.CreateLocale,
		&pb.CreateLocaleRequest{Code: "de-CH", Names: &pb.StringMap{Values: map[string]string{"en": "German (Switzerland)"}}, Formality: unknown},
	)
}
