package export

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRequestedScopeIDs(t *testing.T) {
	for _, tc := range []struct {
		single string
		many   []string
		want   []string
	}{
		{"", nil, []string{}},
		{"a", nil, []string{"a"}},
		{"", []string{"a", "b"}, []string{"a", "b"}},
		{"a", []string{"b", "a", "", "b"}, []string{"a", "b"}},
	} {
		if got := requestedScopeIDs(tc.single, tc.many); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("requestedScopeIDs(%q, %v) = %v, want %v", tc.single, tc.many, got, tc.want)
		}
	}
}

func TestFilterByScopesSQL(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test"}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	statement := filterByScopes(db, db.Model(&models.Entry{}), []string{"scope-1", "scope-2"}).Find(&[]models.Entry{}).Statement
	sql := statement.SQL.String()
	if !strings.Contains(sql, `entries.id IN (SELECT entry_id FROM "entry_scopes" WHERE scope_id IN ($1,$2))`) {
		t.Errorf("expected scope subquery, got %s", sql)
	}
	if strings.Contains(sql, "JOIN") {
		t.Errorf("scope filter must not join (duplicates entries in several scopes): %s", sql)
	}

	unfiltered := filterByScopes(db, db.Model(&models.Entry{}), nil).Find(&[]models.Entry{}).Statement.SQL.String()
	if strings.Contains(unfiltered, "entry_scopes") {
		t.Errorf("no scopes must not filter: %s", unfiltered)
	}
}
