package importservice

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
)

func TestZipImportCreatesMissingLanguages(t *testing.T) {
	f := setupOutdatedFixture(t)

	german := models.Locale{Code: "de-DE"}
	dutch := models.Locale{Code: "nl"}

	for _, row := range []any{&german, &dutch} {
		if err := f.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := f.db.Delete(&dutch).Error; err != nil {
		t.Fatal(err)
	}

	creator, err := testutil.ContextWithPermissions(f.db, "create_entry", "update_entry", "create_locale")
	if err != nil {
		t.Fatal(err)
	}

	archive := zipArchive(t,
		[2]string{"sv.json", `{"a":"A-sv","fresh":"Färsk"}`},
		[2]string{"xa.json", `{"a":"A2","fresh":"Fresh"}`},
		[2]string{"de-de.json", `{"a":"A-de-DE"}`},
		[2]string{"pt_BR.json", `{"a":"A-br"}`},
		[2]string{"nl.json", `{"a":"A-nl"}`},
		[2]string{"readme.json", `{"a":"A"}`},
		[2]string{"frontend/sv.json", `{"b":"B-sv"}`},
	)
	importZip := func(ctx context.Context, dryRun bool, create ...string) (map[string][]*pb.LocaleImportResult, error) {
		t.Helper()

		req := connect.NewRequest(&pb.ImportZipRequest{FileContent: archive, DryRun: dryRun, CreateLocaleCodes: create})
		req.Header().Set("Accept-Language", "de")

		res, err := f.server.ImportZip(ctx, req)
		if err != nil {
			return nil, err
		}

		results := map[string][]*pb.LocaleImportResult{}
		for _, result := range res.Msg.Results {
			results[result.LocaleCode] = append(results[result.LocaleCode], result)
		}

		return results, nil
	}
	localeCount := func() (count int64) {
		f.db.Model(&models.Locale{}).Count(&count)

		return count
	}
	missing := func(name string, result *pb.LocaleImportResult, reason pb.MissingLocaleReason, localeName string) {
		t.Helper()

		if !result.LocaleMissing || result.MissingReason != reason || result.LocaleName != localeName ||
			result.LocaleCreated || result.Result != nil || result.Error == "" {
			t.Errorf("%s: %+v, want missing %s (%s), skipped with an error", name, result, localeName, reason)
		}
	}

	locales := localeCount()

	t.Run("a dry run shows what blocks missing languages", func(t *testing.T) {
		results, err := importZip(f.ctx, true)
		if err != nil {
			t.Fatal(err)
		}

		missing("sv", results["sv"][0], pb.MissingLocaleReason_MISSING_LOCALE_REASON_NO_PERMISSION, "Schwedisch")
		missing("nl", results["nl"][0], pb.MissingLocaleReason_MISSING_LOCALE_REASON_DELETED, "Niederländisch")
		missing("readme", results["readme"][0], pb.MissingLocaleReason_MISSING_LOCALE_REASON_INVALID_CODE, "readme")

		// Another case of an existing code goes into that language.
		if got := results["de-DE"]; len(got) != 1 || got[0].LocaleMissing || got[0].GetResult().GetTranslationsAdded() != 1 {
			t.Errorf("de-de.json: %+v, want it imported into de-DE", got)
		}
	})

	t.Run("a dry run imports into the languages it may create", func(t *testing.T) {
		results, err := importZip(creator, true)
		if err != nil {
			t.Fatal(err)
		}

		swedish := results["sv"]
		if len(swedish) != 2 {
			t.Fatalf("sv: %+v, want both files", swedish)
		}

		for i, want := range []int32{2, 1} {
			if result := swedish[i]; !result.LocaleMissing || result.MissingReason != pb.MissingLocaleReason_MISSING_LOCALE_REASON_UNSPECIFIED ||
				result.LocaleName != "Schwedisch" || result.LocaleCreated || result.GetResult().GetTranslationsAdded() != want {
				t.Errorf("sv file %d: %+v, want it imported as a new language, adding %d", i, result, want)
			}
		}

		if brazilian := results["pt_BR"][0]; !brazilian.LocaleMissing || brazilian.LocaleName != "Brasilianisches Portugiesisch" || brazilian.Result == nil {
			t.Errorf("pt_BR: %+v, want it imported as a new language", brazilian)
		}

		if got := localeCount(); got != locales {
			t.Errorf("a dry run left %d locales, want %d", got, locales)
		}
	})

	t.Run("creating languages needs the permission", func(t *testing.T) {
		if _, err := importZip(f.ctx, false, "sv"); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("ImportZip() error = %v, want permission denied", err)
		}
	})

	t.Run("an import creates the listed languages only", func(t *testing.T) {
		results, err := importZip(creator, false, "SV")
		if err != nil {
			t.Fatal(err)
		}

		for i, result := range results["sv"] {
			if !result.LocaleCreated || result.LocaleMissing || result.LocaleName != "Schwedisch" || result.Result == nil {
				t.Errorf("sv file %d: %+v, want it imported into the created language", i, result)
			}
		}

		missing("pt_BR", results["pt_BR"][0], pb.MissingLocaleReason_MISSING_LOCALE_REASON_UNSPECIFIED, "Brasilianisches Portugiesisch")

		var swedish models.Locale
		if err := f.db.Where("code = ?", "sv").First(&swedish).Error; err != nil {
			t.Fatalf("load sv: %v", err)
		}

		if swedish.IsDefault {
			t.Error("sv became the default language")
		}

		var texts int64

		f.db.Model(&models.Localization{}).Where("locale_id = ?", swedish.ID).Count(&texts)

		if texts != 3 {
			t.Errorf("sv has %d texts, want 3 from both files", texts)
		}

		if got := localeCount(); got != locales+1 {
			t.Errorf("%d locales, want only sv added to %d", got, locales)
		}
	})
}
