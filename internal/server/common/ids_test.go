package common

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestUniqueIDs(t *testing.T) {
	for _, tc := range []struct {
		ids  []string
		want []string
	}{
		{nil, []string{}},
		{[]string{"a", "b"}, []string{"a", "b"}},
		{[]string{"b", "a", "", "b"}, []string{"b", "a"}},
	} {
		if got := UniqueIDs(tc.ids); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("UniqueIDs(%v) = %v, want %v", tc.ids, got, tc.want)
		}
	}
}

func TestFilterEntriesByScopesSQL(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test"}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}

	sql := FilterEntriesByScopes(db.Model(&models.Entry{}), []string{"scope-1", "scope-2"}).Find(&[]models.Entry{}).Statement.SQL.String()
	if !strings.Contains(sql, "entries.id IN (SELECT entry_id FROM entry_scopes WHERE scope_id IN ($1,$2))") {
		t.Errorf("expected scope subquery, got %s", sql)
	}

	if strings.Contains(sql, "JOIN") {
		t.Errorf("scope filter must not join (duplicates entries in several scopes): %s", sql)
	}

	unfiltered := FilterEntriesByScopes(db.Model(&models.Entry{}), nil).Find(&[]models.Entry{}).Statement.SQL.String()
	if strings.Contains(unfiltered, "entry_scopes") {
		t.Errorf("no scopes must not filter: %s", unfiltered)
	}
}
