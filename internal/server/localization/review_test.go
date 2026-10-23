package localization

import (
	"context"
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/webhook"
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

func TestApprovingQueuesSubscribedTasks(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}
	seedReviewData(t, server)

	task := models.Task{Name: "Ship", Type: models.TaskTypeGitExport, Enabled: true, Events: []string{models.EventTranslationsApproved}}
	db.Create(&task)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM task_runs WHERE task_id = ?`, task.ID)
		db.Exec(`DELETE FROM tasks WHERE id = ?`, task.ID)
	})

	queued := func() int64 {
		var count int64

		db.Model(&models.TaskRun{}).Where("task_id = ? AND status = ?", task.ID, models.RunQueued).Count(&count)

		return count
	}

	if _, err := review(ctx, server, &pb.ReviewLocalizationsRequest{
		LocalizationIds: []string{"rv-l1"}, Decision: pb.ReviewDecision_REVIEW_DECISION_REJECT, Comment: "Typo",
	}); err != nil || queued() != 0 {
		t.Fatalf("rejecting: err = %v, %d queued runs, want none", err, queued())
	}

	if _, err := review(ctx, server, &pb.ReviewLocalizationsRequest{
		LocalizationIds: []string{"rv-l1", "rv-l2"}, Decision: pb.ReviewDecision_REVIEW_DECISION_APPROVE,
	}); err != nil || queued() != 1 {
		t.Fatalf("approving: err = %v, %d queued runs, want 1", err, queued())
	}

	// Nothing left to approve emits nothing new; the queued run stays the one.
	if _, err := review(ctx, server, &pb.ReviewLocalizationsRequest{
		LocalizationIds: []string{"rv-l1"}, Decision: pb.ReviewDecision_REVIEW_DECISION_APPROVE,
	}); err != nil || queued() != 1 {
		t.Errorf("approving again: err = %v, %d queued runs, want 1", err, queued())
	}
}

func TestReviewsAreSentToWebhooks(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}
	seedReviewData(t, server)

	hook := models.Webhook{
		Name: "Chat", URL: "https://chat.example/hook", Enabled: true,
		Events: []string{models.WebhookEventTranslationsApproved, models.WebhookEventTranslationsRejected},
	}
	db.Create(&hook)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM webhook_deliveries WHERE webhook_id = ?`, hook.ID)
		db.Exec(`DELETE FROM webhooks WHERE id = ?`, hook.ID)
	})

	sent := func(t *testing.T, event string) webhook.TranslationsData {
		t.Helper()

		var deliveries []models.WebhookDelivery

		db.Where("webhook_id = ? AND event = ?", hook.ID, event).Find(&deliveries)

		if len(deliveries) != 1 {
			t.Fatalf("%d %s deliveries, want 1", len(deliveries), event)
		}

		var body struct {
			Data webhook.TranslationsData `json:"data"`
		}

		if err := json.Unmarshal([]byte(deliveries[0].Payload), &body); err != nil {
			t.Fatalf("payload: %v", err)
		}

		return body.Data
	}

	if _, err := review(ctx, server, &pb.ReviewLocalizationsRequest{
		LocalizationIds: []string{"rv-l1"}, Decision: pb.ReviewDecision_REVIEW_DECISION_REJECT, Comment: " Typo ",
	}); err != nil {
		t.Fatalf("rejecting: %v", err)
	}

	rejected := sent(t, models.WebhookEventTranslationsRejected)
	if rejected.Count != 1 || rejected.Comment != "Typo" || rejected.Translations[0].Key != "rv.one" ||
		rejected.Translations[0].Locale != "rv-de" || rejected.User == nil || rejected.User.ID != middleware.UserID(ctx) {
		t.Errorf("rejected = %+v", rejected)
	}

	if _, err := review(ctx, server, &pb.ReviewLocalizationsRequest{
		EntryIds: []string{"rv-1", "rv-2"}, LocaleIds: []string{"rv-de"}, Decision: pb.ReviewDecision_REVIEW_DECISION_APPROVE,
	}); err != nil {
		t.Fatalf("approving: %v", err)
	}

	approved := sent(t, models.WebhookEventTranslationsApproved)
	if approved.Count != 2 || len(approved.Translations) != 2 || approved.Comment != "" || approved.Translations[1].Translation != "Zwei" {
		t.Errorf("approved = %+v", approved)
	}

	// Approving what is approved already changes nothing, so nothing is sent.
	if _, err := review(ctx, server, &pb.ReviewLocalizationsRequest{
		LocalizationIds: []string{"rv-l1"}, Decision: pb.ReviewDecision_REVIEW_DECISION_APPROVE,
	}); err != nil {
		t.Fatalf("approving again: %v", err)
	}

	sent(t, models.WebhookEventTranslationsApproved)
}
