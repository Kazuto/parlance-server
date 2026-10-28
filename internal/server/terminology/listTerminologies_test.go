package terminology

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestListTerminologiesSearch(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}

	testutil.Exec(t, db, `INSERT INTO terminologies (id, term, description) VALUES
		('ts-1', 'Workspace', 'Where a team keeps its projects'),
		('ts-2', 'Board', NULL),
		('ts-3', 'Ticket', 'A unit of work'),
		('ts-4', 'Archive', NULL)`)
	testutil.Exec(t, db, `INSERT INTO definitions (id, terminology_id, locale_id, translation, deleted_at) VALUES
		('ts-d1', 'ts-1', 'ts-de', 'Arbeitsbereich', NULL),
		('ts-d2', 'ts-2', 'ts-de', 'Tafel', NULL),
		('ts-d3', 'ts-2', 'ts-fr', 'Tableau', NULL),
		('ts-d4', 'ts-4', 'ts-de', 'Ablage', '2026-01-01 00:00:00')`)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM definitions WHERE id LIKE 'ts-%'`)
		db.Exec(`DELETE FROM terminologies WHERE id LIKE 'ts-%'`)
	})

	search := func(term string, localeIDs ...string) []string {
		t.Helper()

		res, err := server.ListTerminologies(ctx, connect.NewRequest(&pb.ListTerminologiesRequest{
			Filter:          &pb.FilterRequest{Search: term},
			TargetLocaleIds: localeIDs,
		}))
		if err != nil {
			t.Fatalf("ListTerminologies: %v", err)
		}

		terms := []string{}

		for _, terminology := range res.Msg.Terminologies {
			if strings.HasPrefix(terminology.Id, "ts-") {
				terms = append(terms, terminology.Term)
			}
		}

		return terms
	}

	cases := []struct {
		name      string
		term      string
		localeIDs []string
		want      []string
	}{
		{"matches the term ignoring case", "WORKSP", []string{"ts-de"}, []string{"Workspace"}},
		{"matches the description", "unit of", nil, []string{"Ticket"}},
		{"matches definitions in the selected locales", "arbeits", []string{"ts-de"}, []string{"Workspace"}},
		{"ignores definitions in other locales", "tableau", []string{"ts-de"}, []string{}},
		{"matches each selected locale", "ta", []string{"ts-de", "ts-fr"}, []string{"Board"}},
		{"searches every locale when none are selected", "tableau", nil, []string{"Board"}},
		{"ignores deleted definitions", "ablage", nil, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := search(tc.term, tc.localeIDs...)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("search %q in %v: got %v, want %v", tc.term, tc.localeIDs, got, tc.want)
			}
		})
	}
}
