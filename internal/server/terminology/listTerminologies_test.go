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

func TestListTerminologiesByStatus(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}

	testutil.Exec(t, db, `INSERT INTO locales (id, code, is_default, deleted_at) VALUES
		('gs-en', 'gs-en', 1, NULL), ('gs-de', 'gs-de', 0, NULL), ('gs-gone', 'gs-gone', 0, '2026-01-01 00:00:00')`)
	testutil.Exec(t, db, `INSERT INTO terminologies (id, term) VALUES
		('gs-1', 'Approved'), ('gs-2', 'Draft'), ('gs-3', 'No German'), ('gs-4', 'No English'),
		('gs-5', 'Blank German'), ('gs-6', 'Deleted German')`)
	// A deleted locale's definitions don't count.
	testutil.Exec(t, db, `INSERT INTO definitions (id, terminology_id, locale_id, translation, status, deleted_at) VALUES
		('gs-d1', 'gs-1', 'gs-en', 'Approved', 'approved', NULL),
		('gs-d2', 'gs-1', 'gs-de', 'Freigegeben', 'approved', NULL),
		('gs-d3', 'gs-2', 'gs-en', 'Draft', 'approved', NULL),
		('gs-d4', 'gs-2', 'gs-de', 'Entwurf', 'draft', NULL),
		('gs-d5', 'gs-3', 'gs-en', 'No German', 'draft', NULL),
		('gs-d6', 'gs-4', 'gs-de', 'Kein Englisch', 'approved', NULL),
		('gs-d7', 'gs-5', 'gs-en', 'Blank German', 'approved', NULL),
		('gs-d8', 'gs-5', 'gs-de', '  ', 'approved', NULL),
		('gs-d9', 'gs-6', 'gs-en', 'Deleted German', 'approved', NULL),
		('gs-d10', 'gs-6', 'gs-de', 'Gelöscht', 'draft', '2026-01-01 00:00:00'),
		('gs-d11', 'gs-1', 'gs-gone', 'Weg', 'draft', NULL)`)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM definitions WHERE id LIKE 'gs-%'`)
		db.Exec(`DELETE FROM terminologies WHERE id LIKE 'gs-%'`)
		db.Exec(`DELETE FROM locales WHERE id LIKE 'gs-%'`)
	})

	list := func(status string, localeIDs ...string) []string {
		t.Helper()

		res, err := server.ListTerminologies(ctx, connect.NewRequest(&pb.ListTerminologiesRequest{
			TranslationStatus: status,
			TargetLocaleIds:   localeIDs,
		}))
		if err != nil {
			t.Fatalf("ListTerminologies(%q): %v", status, err)
		}

		terms := []string{}

		for _, terminology := range res.Msg.Terminologies {
			if strings.HasPrefix(terminology.Id, "gs-") {
				terms = append(terms, terminology.Term)
			}
		}

		return terms
	}

	cases := []struct {
		name      string
		status    string
		localeIDs []string
		want      []string
	}{
		{"all", "", nil, []string{"Approved", "Blank German", "Deleted German", "Draft", "No English", "No German"}},
		{"missing in German", "missing", []string{"gs-de"}, []string{"Blank German", "Deleted German", "No German"}},
		{"missing in an active locale", "missing", nil, []string{"Blank German", "Deleted German", "No English", "No German"}},
		{"complete in German", "complete", []string{"gs-de"}, []string{"Approved", "Draft", "No English"}},
		{"complete, the default locale included", "complete", nil, []string{"Approved", "Draft"}},
		{"drafts in German", "review", []string{"gs-de"}, []string{"Draft"}},
		{"drafts in an active locale", "review", nil, []string{"Draft", "No German"}},
		{"drafts in English", "review", []string{"gs-en"}, []string{"No German"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := list(tc.status, tc.localeIDs...)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("status %q in %v: got %v, want %v", tc.status, tc.localeIDs, got, tc.want)
			}
		})
	}

	for _, status := range []string{"outdated", "check", "unknown"} {
		_, err := server.ListTerminologies(ctx, connect.NewRequest(&pb.ListTerminologiesRequest{TranslationStatus: status}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("status %q: expected invalid argument, got %v", status, err)
		}
	}
}
