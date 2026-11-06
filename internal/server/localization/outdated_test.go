package localization

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"google.golang.org/protobuf/proto"
)

func TestOutdatedTranslations(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}

	// English is the default; od-2's German shows changes stay within the entry.
	seed := func(t *testing.T) {
		t.Helper()

		db.Exec(`DELETE FROM localization_history WHERE entry_id LIKE 'od-%'`)
		db.Exec(`DELETE FROM localizations WHERE entry_id LIKE 'od-%'`)
		testutil.Exec(t, db, `INSERT INTO localizations (id, entry_id, locale_id, translation, status) VALUES
			('od-en', 'od-1', 'od-en', 'Hello', 'approved'),
			('od-de', 'od-1', 'od-de', 'Hallo', 'approved'),
			('od-fr', 'od-1', 'od-fr', 'Bonjour', 'draft'),
			('od-other', 'od-2', 'od-de', 'Tschüss', 'approved')`)
	}

	testutil.Exec(t, db, `INSERT INTO locales (id, code, is_default) VALUES ('od-en', 'od-en', 1), ('od-de', 'od-de', 0), ('od-fr', 'od-fr', 0)`)
	testutil.Exec(t, db, `INSERT INTO entries (id, key) VALUES ('od-1', 'od.one'), ('od-2', 'od.two')`)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM localization_history WHERE entry_id LIKE 'od-%'`)
		db.Exec(`DELETE FROM localizations WHERE entry_id LIKE 'od-%'`)
		db.Exec(`DELETE FROM entries WHERE id LIKE 'od-%'`)
		db.Exec(`DELETE FROM locales WHERE id LIKE 'od-%'`)
	})

	load := func(t *testing.T, id string) models.Localization {
		t.Helper()

		var localization models.Localization
		if err := db.First(&localization, "id = ?", id).Error; err != nil {
			t.Fatalf("load %s: %v", id, err)
		}

		return localization
	}
	outdated := func(t *testing.T, ids ...string) []string {
		t.Helper()

		var marked []string

		for _, id := range ids {
			if load(t, id).OutdatedAt != nil {
				marked = append(marked, id)
			}
		}

		return marked
	}
	edit := func(t *testing.T, id, text string) *pb.Localization {
		t.Helper()

		res, err := server.UpdateLocalization(ctx, connect.NewRequest(&pb.UpdateLocalizationRequest{Id: id, Translation: proto.String(text)}))
		if err != nil {
			t.Fatalf("UpdateLocalization(%s): %v", id, err)
		}

		return res.Msg.Localization
	}
	mark := func(t *testing.T, ids ...string) {
		t.Helper()

		if err := db.Model(&models.Localization{}).Where("id IN ?", ids).UpdateColumn("outdated_at", time.Now()).Error; err != nil {
			t.Fatal(err)
		}
	}
	all := []string{"od-en", "od-de", "od-fr", "od-other"}

	t.Run("a new default text marks the entry's other translations", func(t *testing.T) {
		seed(t)

		source := edit(t, "od-en", "Hi")

		if got := outdated(t, all...); len(got) != 2 || got[0] != "od-de" || got[1] != "od-fr" {
			t.Errorf("outdated = %v, want [od-de od-fr]", got)
		}

		if source.Outdated || source.OutdatedAt != "" {
			t.Errorf("default text = %+v, want it current", source)
		}

		// The mark leaves the text, its review and its history alone.
		german := load(t, "od-de")
		if german.Translation != "Hallo" || german.Status != models.StatusApproved {
			t.Errorf("German = %+v, want its approved text", german)
		}

		var history int64

		db.Table("localization_history").Where("localization_id IN ?", []string{"od-de", "od-fr"}).Count(&history)

		if history != 0 {
			t.Errorf("%d history rows for the marked translations, want none", history)
		}
	})

	t.Run("later changes keep the time of the first", func(t *testing.T) {
		seed(t)
		edit(t, "od-en", "Hi")

		first := load(t, "od-de").OutdatedAt

		edit(t, "od-en", "Hey")

		if later := load(t, "od-de").OutdatedAt; later == nil || !later.Equal(*first) {
			t.Errorf("outdated at %v after the second change, want %v", later, first)
		}
	})

	t.Run("saving the same default text marks nothing", func(t *testing.T) {
		seed(t)
		edit(t, "od-en", "Hello")

		if got := outdated(t, all...); len(got) != 0 {
			t.Errorf("outdated = %v, want none", got)
		}
	})

	t.Run("editing a translation makes it current", func(t *testing.T) {
		seed(t)
		mark(t, "od-de", "od-fr")

		german := edit(t, "od-de", "Hallo!")

		if got := outdated(t, "od-de", "od-fr"); len(got) != 1 || got[0] != "od-fr" {
			t.Errorf("outdated = %v, want [od-fr]", got)
		}

		if german.Outdated {
			t.Errorf("edited translation = %+v, want it current", german)
		}
	})

	t.Run("a default text created later marks the translations", func(t *testing.T) {
		seed(t)
		db.Exec(`DELETE FROM localizations WHERE id = 'od-en'`)

		_, err := server.CreateLocalization(ctx, connect.NewRequest(&pb.CreateLocalizationRequest{
			EntryId:     "od-1",
			LocaleId:    "od-en",
			Translation: "Hi",
		}))
		if err != nil {
			t.Fatalf("CreateLocalization: %v", err)
		}

		if got := outdated(t, "od-de", "od-fr", "od-other"); len(got) != 2 || got[0] != "od-de" || got[1] != "od-fr" {
			t.Errorf("outdated = %v, want [od-de od-fr]", got)
		}
	})

	approve := func(t *testing.T, confirm bool) []*pb.Localization {
		t.Helper()

		reviewed, err := review(ctx, server, &pb.ReviewLocalizationsRequest{
			EntryIds:        []string{"od-1"},
			LocaleIds:       []string{"od-de", "od-fr"},
			Decision:        pb.ReviewDecision_REVIEW_DECISION_APPROVE,
			ConfirmOutdated: confirm,
		})
		if err != nil {
			t.Fatalf("ReviewLocalizations: %v", err)
		}

		return reviewed
	}

	t.Run("approving a draft makes it current, leaving approved ones outdated", func(t *testing.T) {
		seed(t)
		mark(t, "od-de", "od-fr")

		reviewed := approve(t, false)

		if len(reviewed) != 1 || reviewed[0].Id != "od-fr" || reviewed[0].Outdated {
			t.Errorf("reviewed = %+v, want only the current French", reviewed)
		}

		if got := outdated(t, "od-de", "od-fr"); len(got) != 1 || got[0] != "od-de" {
			t.Errorf("outdated = %v, want [od-de]", got)
		}

		if german := load(t, "od-de"); german.ReviewedAt != nil {
			t.Errorf("German reviewed at %v, want it untouched", german.ReviewedAt)
		}
	})

	t.Run("confirming approves outdated translations again", func(t *testing.T) {
		seed(t)
		mark(t, "od-de")

		reviewed := approve(t, true)

		ids := map[string]bool{}
		for _, localization := range reviewed {
			ids[localization.Id] = !localization.Outdated && localization.Status == pb.ReviewStatus_REVIEW_STATUS_APPROVED
		}

		if len(ids) != 2 || !ids["od-de"] || !ids["od-fr"] {
			t.Errorf("reviewed = %+v, want the German and French approved and current", reviewed)
		}

		if got := outdated(t, "od-de"); len(got) != 0 {
			t.Errorf("outdated = %v, want none", got)
		}

		var actions []string

		db.Table("localization_history").Where("localization_id = ?", "od-de").Pluck("action", &actions)

		if len(actions) != 1 || actions[0] != models.ActionApproved {
			t.Errorf("German history = %v, want [approved]", actions)
		}
	})

	t.Run("confirming leaves current approved translations alone", func(t *testing.T) {
		seed(t)

		if reviewed := approve(t, true); len(reviewed) != 1 || reviewed[0].Id != "od-fr" {
			t.Errorf("reviewed = %+v, want only the French draft", reviewed)
		}
	})

	t.Run("rejecting keeps the mark", func(t *testing.T) {
		seed(t)
		mark(t, "od-fr")

		_, err := review(ctx, server, &pb.ReviewLocalizationsRequest{
			LocalizationIds: []string{"od-fr"},
			Decision:        pb.ReviewDecision_REVIEW_DECISION_REJECT,
			Comment:         "Check the new English",
		})
		if err != nil {
			t.Fatalf("ReviewLocalizations: %v", err)
		}

		if got := outdated(t, "od-fr"); len(got) != 1 {
			t.Errorf("outdated = %v, want [od-fr]", got)
		}
	})
}
