package common

import (
	"context"
	"strings"
	"testing"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/gorm"
)

func filterSQL(t *testing.T, ctx context.Context, filter *pb.FilterRequest) string {
	t.Helper()
	_, db := testutil.Setup(t)

	if ctx == nil {
		var err error
		if ctx, err = testutil.ContextWithPermissions(db); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
	}

	query, err := NewFilter(db.Session(&gorm.Session{DryRun: true}).Model(&models.Scope{}), filter).
		IncludeDeletedRequires(ctx, db, permissions.DeleteScope).
		DefaultSort("name ASC").
		Apply()
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	var scopes []models.Scope

	return query.Find(&scopes).Statement.SQL.String()
}

func TestFilterAppliesDefaultSortWithoutFilter(t *testing.T) {
	if sql := filterSQL(t, nil, nil); !strings.Contains(sql, "ORDER BY name ASC") {
		t.Errorf("expected default sort, got %s", sql)
	}
}

func TestFilterIgnoresIncludeDeletedWithoutPermission(t *testing.T) {
	sql := filterSQL(t, nil, &pb.FilterRequest{IncludeDeleted: true})
	if !strings.Contains(sql, "deleted_at") {
		t.Errorf("expected soft-deleted rows to stay hidden, got %s", sql)
	}
}

func TestFilterIncludesDeletedWithPermission(t *testing.T) {
	ctx, db := testutil.Setup(t)

	query, err := NewFilter(db.Session(&gorm.Session{DryRun: true}).Model(&models.Scope{}), &pb.FilterRequest{IncludeDeleted: true}).
		IncludeDeletedRequires(ctx, db, permissions.DeleteScope).
		Apply()
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	var scopes []models.Scope
	if sql := query.Find(&scopes).Statement.SQL.String(); strings.Contains(sql, "deleted_at") {
		t.Errorf("expected soft-deleted rows to be included, got %s", sql)
	}
}
