package models

import (
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestHistoryHooksRecordActionAuthor(t *testing.T) {
	creator, updater := "creator-id", "updater-id"
	for _, kind := range []string{"localization", "definition"} {
		for _, action := range []string{"created", "updated", "deleted"} {
			t.Run(kind+"/"+action, func(t *testing.T) {
				db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test", PreferSimpleProtocol: true}), &gorm.Config{
					DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true,
				})
				if err != nil {
					t.Fatal(err)
				}
				var recordedUser *string
				var recordedAction string
				if err := db.Callback().Create().Before("gorm:create").Register("test:capture-history", func(tx *gorm.DB) {
					switch history := tx.Statement.Dest.(type) {
					case *LocalizationHistory:
						recordedUser, recordedAction = history.UserID, history.Action
					case *DefinitionHistory:
						recordedUser, recordedAction = history.UserID, history.Action
					}
				}); err != nil {
					t.Fatal(err)
				}
				if kind == "localization" {
					model := &Localization{CreatedBy: &creator, UpdatedBy: &updater}
					switch action {
					case "created":
						err = model.AfterCreate(db)
					case "updated":
						err = model.AfterUpdate(db)
					case "deleted":
						err = model.AfterDelete(db)
					}
				} else {
					model := &Definition{CreatedBy: &creator, UpdatedBy: &updater}
					switch action {
					case "created":
						err = model.AfterCreate(db)
					case "updated":
						err = model.AfterUpdate(db)
					case "deleted":
						err = model.AfterDelete(db)
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				want := updater
				if action == "created" {
					want = creator
				}
				if recordedUser == nil || *recordedUser != want || recordedAction != action {
					t.Fatalf("history author/action = %v/%s, want %s/%s", recordedUser, recordedAction, want, action)
				}
			})
		}
	}
}
