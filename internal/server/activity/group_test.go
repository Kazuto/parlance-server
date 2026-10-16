package activity

import (
	"testing"
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

var base = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func translation(minutesAgo float64, action, user, locale, entry string) change {
	return change{
		ID:        entry + action,
		Kind:      pb.ActivityKind_ACTIVITY_KIND_TRANSLATION,
		Action:    action,
		UserID:    user,
		LocaleID:  locale,
		SubjectID: entry,
		ChangedAt: base.Add(-time.Duration(minutesAgo * float64(time.Minute))),
	}
}

func TestGroupChangesFoldsAnImportIntoOneItem(t *testing.T) {
	var changes []change
	for i := range 300 {
		action := "updated"
		if i%3 == 0 {
			action = "created"
		}
		// 300 rows within a few seconds, newest first.
		changes = append(changes, translation(float64(i)/100, action, "jane", "de", "entry-"+string(rune('a'+i%26))+string(rune('a'+i/26))))
	}
	groups := groupChanges(changes, 10)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if g.Created != 100 || g.Updated != 200 || g.Deleted != 0 {
		t.Errorf("counts = %d/%d/%d, want 100/200/0", g.Created, g.Updated, g.Deleted)
	}
	if len(g.Subjects) != 300 {
		t.Errorf("subjects = %d, want 300", len(g.Subjects))
	}
	if !g.Newest.ChangedAt.Equal(base) || !g.Oldest.Equal(base.Add(-time.Duration(2.99*float64(time.Minute)))) {
		t.Errorf("range = %v..%v", g.Oldest, g.Newest.ChangedAt)
	}
}

func TestGroupChangesSplits(t *testing.T) {
	for name, second := range map[string]change{
		"after a long pause": translation(6, "updated", "jane", "de", "b"),
		"for another user":   translation(1, "updated", "john", "de", "b"),
		"for another kind":   {Kind: pb.ActivityKind_ACTIVITY_KIND_DEFINITION, Action: "updated", UserID: "jane", LocaleID: "de", SubjectID: "term", ChangedAt: base.Add(-time.Minute)},
	} {
		groups := groupChanges([]change{translation(0, "updated", "jane", "de", "a"), second}, 10)
		if len(groups) != 2 {
			t.Errorf("%s: expected 2 groups, got %d", name, len(groups))
		}
	}
}

func TestGroupChangesCountsLocales(t *testing.T) {
	// Filling one key in three languages is one item.
	changes := []change{
		translation(0, "created", "jane", "de", "a"),
		translation(0.5, "created", "jane", "fr", "a"),
		translation(1, "created", "jane", "es", "a"),
	}
	groups := groupChanges(changes, 10)
	if len(groups) != 1 || len(groups[0].Locales) != 3 || len(groups[0].Subjects) != 1 {
		t.Fatalf("expected one group for 1 entry in 3 locales, got %+v", groups)
	}
}

func TestGroupChangesKeepsGroupingWhileGapsStaySmall(t *testing.T) {
	// Each change is 4 minutes after the previous one: one session spanning 12 minutes.
	changes := []change{
		translation(0, "updated", "jane", "de", "a"),
		translation(4, "updated", "jane", "de", "a"),
		translation(8, "updated", "jane", "de", "b"),
		translation(12, "updated", "jane", "de", "a"),
	}
	groups := groupChanges(changes, 10)
	if len(groups) != 1 || groups[0].Updated != 4 || len(groups[0].Subjects) != 2 {
		t.Fatalf("expected one group with 4 updates on 2 entries, got %+v", groups)
	}
}

func TestGroupChangesStopsAtTheLimit(t *testing.T) {
	changes := []change{
		translation(0, "updated", "jane", "de", "a"),
		translation(10, "updated", "john", "de", "b"),
		translation(20, "updated", "jane", "fr", "c"),
	}
	groups := groupChanges(changes, 2)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[1].Newest.UserID != "john" {
		t.Errorf("second group = %s, want john", groups[1].Newest.UserID)
	}
}
