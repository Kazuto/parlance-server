package entry

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestTranslationStatusSQL(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test"}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"missing", "complete"} {
		query := filterTranslationStatus(db.Model(&models.Entry{}).Where("key LIKE ?", "%save%"), status, []string{"locale-1"})
		statement := query.Find(&[]models.Entry{}).Statement
		sql := statement.SQL.String()
		for _, part := range []string{"FROM \"locales\"", "localizations.entry_id = entries.id", "localizations.deleted_at IS NULL", "TRIM(localizations.translation)", "locales.id IN", "\"locales\".\"deleted_at\" IS NULL"} {
			if !strings.Contains(sql, part) {
				t.Errorf("missing %q in %s", part, sql)
			}
		}
		if status == "complete" && !strings.Contains(sql, "NOT EXISTS (SELECT 1 FROM \"locales\"") {
			t.Errorf("complete must exclude missing targets: %s", sql)
		}
	}
}

func TestScopeFilterSQL(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test"}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	statement := filterScopes(db.Model(&models.Entry{}), []string{"scope-1", "scope-2"}).Find(&[]models.Entry{}).Statement
	sql := statement.SQL.String()
	if !strings.Contains(sql, "entries.id IN (SELECT entry_id FROM entry_scopes WHERE scope_id IN ($1,$2))") {
		t.Errorf("expected scope subquery, got %s", sql)
	}
	if strings.Contains(sql, "JOIN") {
		t.Errorf("scope filter must not join (duplicates entries in several scopes): %s", sql)
	}
	if len(statement.Vars) != 2 {
		t.Errorf("expected 2 scope vars, got %v", statement.Vars)
	}
}

func TestInvalidTranslationStatus(t *testing.T) {
	_, err := (&Server{}).ListEntries(context.Background(), connect.NewRequest(&pb.ListEntriesRequest{TranslationStatus: "invalid"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}
