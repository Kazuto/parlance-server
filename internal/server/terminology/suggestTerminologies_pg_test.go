package terminology

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
	"gorm.io/datatypes"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func TestSuggestTerminologiesMatchesWordForms(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	if err := db.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ctx, err := testutil.ContextWithPermissions(db, "read_terminology")
	if err != nil {
		t.Fatal(err)
	}

	en := models.Locale{Code: "en", IsDefault: true, Names: datatypes.JSON(`{"en":"English"}`)}
	de := models.Locale{Code: "de", Names: datatypes.JSON(`{"en":"German"}`)}
	scope := models.Terminology{Term: "scope"}

	for _, row := range []any{&en, &de, &scope} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Create(&models.Definition{TerminologyID: scope.ID, LocaleID: de.ID, Translation: "Bereich", Review: models.Review{Status: models.StatusApproved}}).Error; err != nil {
		t.Fatal(err)
	}

	server := &Server{db: db}
	suggest := func(translation *string) []*pb.TerminologySuggestion {
		t.Helper()

		res, err := server.SuggestTerminologies(ctx, connect.NewRequest(&pb.SuggestTerminologiesRequest{
			SourceText: "No scopes found", TargetLocaleId: de.ID, Translation: translation,
		}))
		if err != nil {
			t.Fatalf("SuggestTerminologies: %v", err)
		}

		return res.Msg.Suggestions
	}

	got := suggest(proto.String("Keine Bereiche gefunden"))
	if len(got) != 1 || got[0].TargetTranslation != "Bereich" || got[0].MatchedWords[0] != "scopes" || !got[0].Used {
		t.Fatalf("suggestions = %+v, want scope in its plural, used in its plural", got)
	}

	if got := suggest(proto.String("Keine Scopes gefunden")); len(got) != 1 || got[0].Used {
		t.Errorf("translation without the term: %+v, want unused", got)
	}

	if got := suggest(nil); len(got) != 1 || got[0].Used {
		t.Errorf("without a translation: %+v, want no use checked", got)
	}
}
