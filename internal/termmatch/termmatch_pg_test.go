package termmatch_test

import (
	"context"
	"slices"
	"testing"

	"github.com/kazuto/parlance-server/internal/server/testutil"
	"github.com/kazuto/parlance-server/internal/termmatch"
)

func TestFind(t *testing.T) {
	db := testutil.SetupPostgresDB(t)
	ctx := context.Background()

	tests := []struct {
		code  string
		terms []string
		text  string
		want  []string // the matched words of each term found, as "term: words"
	}{
		{"en", []string{"scope", "copy"}, "Copied to all scopes", []string{"copy: copied", "scope: scopes"}},
		{"en", []string{"scope"}, "Scoped access", []string{"scope: scoped"}},
		{"en", []string{"scope"}, "A scopeless query", nil},
		{"en", []string{"API token"}, "Your API tokens expire", []string{"API token: api tokens"}},
		{"en", []string{"token"}, "Delete {token}", nil},
		{"de", []string{"Name"}, "Du heißt {name}.", nil},
		{"de", []string{"Name"}, "Du heißt :name.", nil},
		{"de", []string{"Name"}, "Deine Namen: {name}.", []string{"Name: namen"}},
		{"en", []string{"No"}, "No entries", []string{"No: no"}},
		{"de", []string{"Bereich", "E-Mail"}, "Keine Bereiche für die E-Mail-Adresse", []string{"Bereich: bereiche", "E-Mail: e mail"}},
		{"fr", []string{"portée"}, "Aucune portées trouvées", []string{"portée: portées"}},
		{"pt-BR", []string{"escopo"}, "Nenhum escopos", []string{"escopo: escopos"}},
		{"xx", []string{"scope"}, "scopes and scope", []string{"scope: scope"}},
		{"zh", []string{"剪贴板"}, "复制到剪贴板", []string{"剪贴板: 剪贴板"}},
		{"ja", []string{"コピー"}, "クリップボードにコピーしました", []string{"コピー: コピー"}},
	}

	for _, test := range tests {
		terms := make([]termmatch.Term, len(test.terms))
		for i, term := range test.terms {
			terms[i] = termmatch.Term{ID: term, Text: term}
		}

		matcher, err := termmatch.New(ctx, db.DB, test.code, terms)
		if err != nil {
			t.Fatalf("%s: New() error = %v", test.code, err)
		}

		found, err := matcher.Find(test.text)
		if err != nil {
			t.Fatalf("%s: Find() error = %v", test.code, err)
		}

		var got []string

		for id, words := range found {
			for _, w := range words {
				got = append(got, id+": "+w)
			}
		}

		slices.Sort(got)

		if !slices.Equal(got, test.want) {
			t.Errorf("%s %q in %q: got %v, want %v", test.code, test.terms, test.text, got, test.want)
		}
	}
}
