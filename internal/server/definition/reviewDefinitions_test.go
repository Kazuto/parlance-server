package definition

import (
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestReviewDefinitions(t *testing.T) {
	ctx, db := testutil.Setup(t)
	testutil.Exec(t, db, `INSERT INTO definitions (id, terminology_id, locale_id, translation) VALUES
		('rd-1', 'rd-term', 'rd-de', 'Übersicht')`)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM definition_history WHERE definition_id LIKE 'rd-%'`)
		db.Exec(`DELETE FROM definitions WHERE id LIKE 'rd-%'`)
	})

	server := &Server{db: db}

	res, err := server.ReviewDefinitions(ctx, connect.NewRequest(&pb.ReviewDefinitionsRequest{
		TerminologyIds: []string{"rd-term"},
		LocaleIds:      []string{"rd-de"},
		Decision:       pb.ReviewDecision_REVIEW_DECISION_APPROVE,
	}))
	if err != nil {
		t.Fatalf("ReviewDefinitions() error = %v", err)
	}

	if len(res.Msg.Definitions) != 1 || res.Msg.Definitions[0].Status != pb.ReviewStatus_REVIEW_STATUS_APPROVED {
		t.Errorf("reviewed definitions = %+v, want one approved", res.Msg.Definitions)
	}
}
