package definition

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestUpdateDefinitionRejectsEmptyRequiredFields(t *testing.T) {
	ctx, db := testutil.Setup(t)
	testutil.ExpectInvalidArgument(t, ctx, (&Server{db: db}).UpdateDefinition,
		&pb.UpdateDefinitionRequest{Translation: proto.String("Übersicht")},
	)
}

// Clearing a definition in the glossary sends an empty text, which removes it.
func TestUpdateDefinitionDeletesAnEmptiedDefinition(t *testing.T) {
	ctx, db := testutil.Setup(t)
	testutil.Exec(t, db, `INSERT INTO locales (id, code) VALUES ('cd-de', 'cd-de')`)
	testutil.Exec(t, db, `INSERT INTO terminologies (id, term) VALUES ('cd-term', 'Dashboard')`)
	testutil.Exec(t, db, `INSERT INTO definitions (id, terminology_id, locale_id, translation) VALUES
		('cd-1', 'cd-term', 'cd-de', 'Übersicht')`)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM definition_history WHERE terminology_id = 'cd-term'`)
		db.Exec(`DELETE FROM definitions WHERE terminology_id = 'cd-term'`)
		db.Exec(`DELETE FROM terminologies WHERE id = 'cd-term'`)
		db.Exec(`DELETE FROM locales WHERE id = 'cd-de'`)
	})

	server := &Server{db: db}

	res, err := server.UpdateDefinition(ctx, connect.NewRequest(&pb.UpdateDefinitionRequest{
		Id:          "cd-1",
		Translation: proto.String(""),
	}))
	if err != nil {
		t.Fatalf("UpdateDefinition() error = %v", err)
	}

	if res.Msg.Definition != nil {
		t.Errorf("response definition = %+v, want none", res.Msg.Definition)
	}

	// Soft deleted, so its history and a restore stay possible.
	var deleted models.Definition
	if err := db.Unscoped().First(&deleted, "id = ?", "cd-1").Error; err != nil {
		t.Fatalf("load: %v", err)
	}

	if !deleted.DeletedAt.Valid {
		t.Error("definition not deleted")
	}

	// The language can get a definition again.
	if _, err := server.CreateDefinition(ctx, connect.NewRequest(&pb.CreateDefinitionRequest{
		TerminologyId: "cd-term",
		LocaleId:      "cd-de",
		Translation:   "Startseite",
	})); err != nil {
		t.Errorf("CreateDefinition() after clearing: %v", err)
	}
}
