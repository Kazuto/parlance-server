package testutil

import (
	"sync"
	"testing"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm/schema"
)

// The test schema is written by hand, as SQLite cannot run the models' Postgres defaults;
// this keeps it in step with the models.
func TestSchemaMatchesModels(t *testing.T) {
	db, err := SetupTestDB()
	if err != nil {
		t.Fatalf("SetupTestDB() error = %v", err)
	}

	for _, model := range []any{
		&models.Locale{}, &models.Scope{}, &models.User{}, &models.Role{}, &models.Permission{},
		&models.Entry{}, &models.EntryScope{}, &models.Localization{}, &models.LocalizationHistory{},
		&models.Terminology{}, &models.Definition{}, &models.DefinitionHistory{}, &models.TranslationServer{},
	} {
		parsed, err := schema.Parse(model, &sync.Map{}, db.NamingStrategy)
		if err != nil {
			t.Fatalf("parse %T: %v", model, err)
		}

		columns, err := db.Migrator().ColumnTypes(parsed.Table)
		if err != nil {
			t.Errorf("%s: %v", parsed.Table, err)

			continue
		}

		have := map[string]bool{}
		for _, column := range columns {
			have[column.Name()] = true
		}

		for _, field := range parsed.Fields {
			if field.DBName != "" && !have[field.DBName] {
				t.Errorf("table %s lacks column %s", parsed.Table, field.DBName)
			}
		}
	}
}
