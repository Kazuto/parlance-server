package terminology

import (
	"reflect"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestSuggestTerminologiesSkipsPlaceholders(t *testing.T) {
	ctx, db := testutil.Setup(t)
	server := &Server{db: db}

	testutil.Exec(t, db, `INSERT INTO locales (id, code, is_default) VALUES ('sg-de', 'sg-de', 0)`)
	testutil.Exec(t, db, `INSERT INTO terminologies (id, term) VALUES ('sg-1', 'Language')`)
	testutil.Exec(t, db, `INSERT INTO definitions (id, terminology_id, locale_id, translation, status) VALUES
		('sg-d1', 'sg-1', 'sg-de', 'Sprache', 'approved')`)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM definitions WHERE id LIKE 'sg-%'`)
		db.Exec(`DELETE FROM terminologies WHERE id LIKE 'sg-%'`)
		db.Exec(`DELETE FROM locales WHERE id LIKE 'sg-%'`)
	})

	matched := func(source string) [][]string {
		t.Helper()

		res, err := server.SuggestTerminologies(ctx, connect.NewRequest(&pb.SuggestTerminologiesRequest{
			SourceText:     source,
			TargetLocaleId: "sg-de",
		}))
		if err != nil {
			t.Fatalf("SuggestTerminologies(%q): %v", source, err)
		}

		words := [][]string{}
		for _, suggestion := range res.Msg.Suggestions {
			words = append(words, suggestion.MatchedWords)
		}

		return words
	}

	// The placeholder is filled in later and stays untranslated.
	if got := matched("Translated into {language} by :language"); len(got) != 0 {
		t.Errorf("placeholders matched %v, want no terms", got)
	}

	if got, want := matched("Choose a language for {language}"), [][]string{{"language"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("matched %v, want %v", got, want)
	}
}
