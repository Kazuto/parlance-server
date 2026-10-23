package localization

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func seedReviewData(t *testing.T, server *Server) {
	t.Helper()

	db := server.db
	testutil.Exec(t, db, `INSERT INTO locales (id, code, is_default) VALUES ('rv-en', 'rv-en', 0), ('rv-de', 'rv-de', 0)`)
	testutil.Exec(t, db, `INSERT INTO entries (id, key) VALUES ('rv-1', 'rv.one'), ('rv-2', 'rv.two')`)
	testutil.Exec(t, db, `INSERT INTO localizations (id, entry_id, locale_id, translation, status) VALUES
		('rv-l1', 'rv-1', 'rv-de', 'Eins', 'draft'),
		('rv-l2', 'rv-2', 'rv-de', 'Zwei', 'draft'),
		('rv-l3', 'rv-1', 'rv-en', 'One', 'approved')`)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM localization_history WHERE entry_id LIKE 'rv-%'`)
		db.Exec(`DELETE FROM localizations WHERE entry_id LIKE 'rv-%'`)
		db.Exec(`DELETE FROM entries WHERE id LIKE 'rv-%'`)
		db.Exec(`DELETE FROM locales WHERE id LIKE 'rv-%'`)
	})
}

func review(ctx context.Context, server *Server, req *pb.ReviewLocalizationsRequest) ([]*pb.Localization, error) {
	res, err := server.ReviewLocalizations(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}

	return res.Msg.Localizations, nil
}

func TestReviewLocalizations(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}
	seedReviewData(t, server)

	reviewer := middleware.UserID(ctx)

	t.Run("approves by entry and locale, skipping missing translations", func(t *testing.T) {
		reviewed, err := review(ctx, server, &pb.ReviewLocalizationsRequest{
			EntryIds:  []string{"rv-1", "rv-2"},
			LocaleIds: []string{"rv-de"},
			Decision:  pb.ReviewDecision_REVIEW_DECISION_APPROVE,
		})
		if err != nil {
			t.Fatalf("ReviewLocalizations: %v", err)
		}

		if len(reviewed) != 2 {
			t.Fatalf("reviewed %d translations, want 2", len(reviewed))
		}

		for _, localization := range reviewed {
			if localization.Status != pb.ReviewStatus_REVIEW_STATUS_APPROVED || localization.ReviewedBy != reviewer || localization.ReviewedAt == "" {
				t.Errorf("reviewed translation = %+v", localization)
			}
		}

		var actions []string

		db.Table("localization_history").Where("localization_id = ?", "rv-l1").Pluck("action", &actions)

		if len(actions) != 1 || actions[0] != models.ActionApproved {
			t.Errorf("history actions = %v, want only approved", actions)
		}
	})

	t.Run("rejects with a comment, keeping a draft", func(t *testing.T) {
		reviewed, err := review(ctx, server, &pb.ReviewLocalizationsRequest{
			LocalizationIds: []string{"rv-l1"},
			Decision:        pb.ReviewDecision_REVIEW_DECISION_REJECT,
			Comment:         "  Use the formal form  ",
		})
		if err != nil {
			t.Fatalf("ReviewLocalizations: %v", err)
		}

		if len(reviewed) != 1 || reviewed[0].Status != pb.ReviewStatus_REVIEW_STATUS_DRAFT || reviewed[0].ReviewComment != "Use the formal form" {
			t.Errorf("rejected translation = %+v", reviewed)
		}

		var history models.LocalizationHistory

		db.Where("localization_id = ? AND action = ?", "rv-l1", models.ActionRejected).First(&history)

		if history.Comment != "Use the formal form" || history.UserID == nil || *history.UserID != reviewer {
			t.Errorf("rejection history = %+v", history)
		}
	})

	t.Run("approving clears the rejection comment", func(t *testing.T) {
		reviewed, err := review(ctx, server, &pb.ReviewLocalizationsRequest{
			LocalizationIds: []string{"rv-l1"},
			Decision:        pb.ReviewDecision_REVIEW_DECISION_APPROVE,
		})
		if err != nil || len(reviewed) != 1 || reviewed[0].ReviewComment != "" {
			t.Errorf("approved translation = %+v, %v", reviewed, err)
		}
	})

	t.Run("rejects invalid requests", func(t *testing.T) {
		testutil.ExpectInvalidArgument(t, ctx, server.ReviewLocalizations,
			&pb.ReviewLocalizationsRequest{LocalizationIds: []string{"rv-l1"}},
			&pb.ReviewLocalizationsRequest{LocalizationIds: []string{"rv-l1"}, Decision: pb.ReviewDecision_REVIEW_DECISION_REJECT},
			&pb.ReviewLocalizationsRequest{Decision: pb.ReviewDecision_REVIEW_DECISION_APPROVE},
			&pb.ReviewLocalizationsRequest{EntryIds: []string{"rv-1"}, Decision: pb.ReviewDecision_REVIEW_DECISION_APPROVE},
		)
	})
}

func TestChangedTextNeedsReviewAgain(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}
	seedReviewData(t, server)

	update := func(translation string) *pb.Localization {
		t.Helper()

		res, err := server.UpdateLocalization(ctx, connect.NewRequest(&pb.UpdateLocalizationRequest{Id: "rv-l3", Translation: proto.String(translation)}))
		if err != nil {
			t.Fatalf("UpdateLocalization: %v", err)
		}

		return res.Msg.Localization
	}

	if got := update("One"); got.Status != pb.ReviewStatus_REVIEW_STATUS_APPROVED {
		t.Errorf("unchanged text: status %v, want approved", got.Status)
	}

	if got := update("One!"); got.Status != pb.ReviewStatus_REVIEW_STATUS_DRAFT {
		t.Errorf("changed text: status %v, want draft", got.Status)
	}

	res, err := server.CreateLocalization(ctx, connect.NewRequest(&pb.CreateLocalizationRequest{EntryId: "rv-2", LocaleId: "rv-en", Translation: "Two"}))
	if err != nil || res.Msg.Localization.Status != pb.ReviewStatus_REVIEW_STATUS_DRAFT {
		t.Errorf("created translation = %+v, %v; want a draft", res, err)
	}
}
