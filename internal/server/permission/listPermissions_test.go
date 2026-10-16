package permission

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestListPermissionsSearchMatchesNameOrResource(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test"}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	var queries []string
	if err := db.Callback().Query().After("gorm:query").Register("test:capture", func(tx *gorm.DB) {
		queries = append(queries, tx.Statement.SQL.String())
	}); err != nil {
		t.Fatal(err)
	}

	server := &Server{db: &database.DB{DB: db}}
	if _, err := server.ListPermissions(context.Background(), connect.NewRequest(&pb.ListPermissionsRequest{Filter: &pb.FilterRequest{Search: "scope", Sort: "resource"}})); err != nil {
		t.Fatal(err)
	}
	if len(queries) == 0 {
		t.Fatal("no query captured")
	}
	for _, sql := range queries {
		// The OR must be grouped, or deleted permissions matching the resource would be returned.
		if !strings.Contains(sql, "(name ILIKE $1 OR resource ILIKE $2) AND") {
			t.Errorf("expected grouped name/resource search, got %s", sql)
		}
	}
}
