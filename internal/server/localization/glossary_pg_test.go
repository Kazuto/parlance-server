package localization

import (
	"context"
	"testing"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

func TestGlossaryFindsWordForms(t *testing.T) {
	db := testutil.SetupPostgresDB(t)

	de := models.Locale{Code: "de", Names: datatypes.JSON(`{"en":"German"}`)}
	copyTerm := models.Terminology{Term: "copy"}
	draftTerm := models.Terminology{Term: "token"}

	for _, row := range []any{&de, &copyTerm, &draftTerm} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	for _, definition := range []models.Definition{
		{TerminologyID: copyTerm.ID, LocaleID: de.ID, Translation: "kopieren", Review: models.Review{Status: models.StatusApproved}},
		// A draft can't steer machine translations.
		{TerminologyID: draftTerm.ID, LocaleID: de.ID, Translation: "Schlüssel", Review: models.Review{Status: models.StatusDraft}},
	} {
		if err := db.Create(&definition).Error; err != nil {
			t.Fatal(err)
		}
	}

	glossary, err := LoadGlossary(context.Background(), db.DB, "en", []string{de.ID})
	if err != nil {
		t.Fatalf("LoadGlossary: %v", err)
	}

	terms, err := glossary.Terms("Copied the tokens")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}

	if got := terms[de.ID]; len(got) != 1 || got[0].Source != "copy" || got[0].Target != "kopieren" {
		t.Errorf("terms = %+v, want copy → kopieren only", got)
	}
}
