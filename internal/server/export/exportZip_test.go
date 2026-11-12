package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/testutil"
	"gorm.io/datatypes"
)

// selectionFixture holds the scopes, tags and locales seedSelection creates
type selectionFixture struct {
	mail, web     models.Scope
	billing, home models.Tag
	ex, fr        models.Locale
}

// seedSelection creates Mail (PHP by default) with mail.receipt tagged billing and
// mail.welcome; Web (no default, an unsafe slug) with web.home tagged home and
// web.about; and "other" without a scope. Each has an approved text in ex; fr has none.
func seedSelection(t *testing.T, db *database.DB) selectionFixture {
	t.Helper()

	// The package's tests share a database: start from, and leave, empty tables.
	emptyTables := func() {
		for _, table := range []string{"release_entries", "releases", "entry_tags", "entry_scopes", "tags", "scopes", "localizations", "entries", "locales"} {
			db.Exec("DELETE FROM " + table)
		}
	}
	emptyTables()
	t.Cleanup(emptyTables)

	f := selectionFixture{
		mail: models.Scope{Name: "Mail", Slug: "mail", ExportFormat: FormatPHP},
		web:  models.Scope{Name: "Web", Slug: "../web site"},
		ex:   models.Locale{Code: "ex", Names: datatypes.JSON(`{"en":"Export"}`)},
		fr:   models.Locale{Code: "fr", Names: datatypes.JSON(`{"en":"French"}`)},
	}

	create := func(row any) {
		t.Helper()

		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	for _, row := range []any{&f.mail, &f.web, &f.ex, &f.fr} {
		create(row)
	}

	f.billing = models.Tag{ScopeID: f.mail.ID, Name: "billing"}
	f.home = models.Tag{ScopeID: f.web.ID, Name: "home"}
	create(&f.billing)
	create(&f.home)

	for key, links := range map[string]struct {
		scope *models.Scope
		tag   *models.Tag
	}{
		"mail.receipt": {&f.mail, &f.billing},
		"mail.welcome": {&f.mail, nil},
		"web.home":     {&f.web, &f.home},
		"web.about":    {&f.web, nil},
		"other":        {nil, nil},
	} {
		entry := models.Entry{Key: key}
		create(&entry)
		create(&models.Localization{
			EntryID: entry.ID, LocaleID: f.ex.ID, Translation: "text of " + key,
			Review: models.Review{Status: models.StatusApproved},
		})

		if links.scope != nil {
			create(&models.EntryScope{EntryID: entry.ID, ScopeID: links.scope.ID})
		}

		if links.tag != nil {
			create(&models.EntryTag{EntryID: entry.ID, TagID: links.tag.ID, ScopeID: links.tag.ScopeID})
		}
	}

	return f
}

// jsonKeys lists the keys of an exported JSON file
func jsonKeys(t *testing.T, content string) []string {
	t.Helper()

	var translations map[string]string
	if err := json.Unmarshal([]byte(content), &translations); err != nil {
		t.Fatalf("parse %q: %v", content, err)
	}

	return slices.Sorted(maps.Keys(translations))
}

// unzip reads an exported archive's files by name
func unzip(t *testing.T, content []byte) map[string]string {
	t.Helper()

	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("read ZIP: %v", err)
	}

	files := map[string]string{}

	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}

		_ = reader.Close()
		files[file.Name] = string(data)
	}

	return files
}

func TestExportSelection(t *testing.T) {
	ctx, db := testutil.Setup(t)
	f := seedSelection(t, db)
	server := &Server{db: db}

	for name, tc := range map[string]struct {
		scopeIDs, tagIDs []string
		want             []string
	}{
		"scopes":                       {[]string{f.mail.ID, f.web.ID}, nil, []string{"mail.receipt", "mail.welcome", "web.about", "web.home"}},
		"a tag narrows only its scope": {[]string{f.mail.ID, f.web.ID}, []string{f.billing.ID}, []string{"mail.receipt", "web.about", "web.home"}},
		"a tag selects its scope":      {nil, []string{f.billing.ID}, []string{"mail.receipt"}},
		"everything":                   {nil, nil, []string{"mail.receipt", "mail.welcome", "other", "web.about", "web.home"}},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := server.ExportJson(ctx, connect.NewRequest(&pb.ExportJsonRequest{LocaleCode: "ex", ScopeIds: tc.scopeIDs, TagIds: tc.tagIDs}))
			if err != nil {
				t.Fatalf("ExportJson() error = %v", err)
			}

			if got := jsonKeys(t, res.Msg.Content); !slices.Equal(got, tc.want) {
				t.Errorf("keys = %v, want %v", got, tc.want)
			}
		})
	}

	// PHP and PO exports take the same selection.
	php, err := server.ExportPhp(ctx, connect.NewRequest(&pb.ExportPhpRequest{LocaleCode: "ex", TagIds: []string{f.home.ID}}))
	if err != nil || !strings.Contains(php.Msg.Content, "'web.home'") || strings.Contains(php.Msg.Content, "web.about") {
		t.Errorf("PHP export = %v, %v", php, err)
	}

	po, err := server.ExportPo(ctx, connect.NewRequest(&pb.ExportPoRequest{LocaleCode: "ex", ScopeIds: []string{f.mail.ID}, TagIds: []string{f.billing.ID}}))
	if err != nil || !strings.Contains(po.Msg.Content, `"mail.receipt"`) || strings.Contains(po.Msg.Content, "mail.welcome") {
		t.Errorf("PO export = %v, %v", po, err)
	}

	testutil.ExpectInvalidArgument(t, ctx, server.ExportJson,
		&pb.ExportJsonRequest{LocaleCode: "ex", TagIds: []string{"missing"}},
		&pb.ExportJsonRequest{LocaleCode: "ex", ScopeIds: []string{"missing"}},
	)

	// A deleted scope's tags select nothing.
	if err := db.Delete(&f.web).Error; err != nil {
		t.Fatal(err)
	}

	testutil.ExpectInvalidArgument(t, ctx, server.ExportJson, &pb.ExportJsonRequest{LocaleCode: "ex", TagIds: []string{f.home.ID}})
}

func TestExportZipFolderPerScope(t *testing.T) {
	ctx, db := testutil.Setup(t)
	f := seedSelection(t, db)
	server := &Server{db: db}

	res, err := server.ExportZip(ctx, connect.NewRequest(&pb.ExportZipRequest{
		ScopeIds:  []string{f.web.ID, f.mail.ID},
		TagIds:    []string{f.billing.ID},
		LocaleIds: []string{f.ex.ID},
		Format:    pb.ExportFormat_EXPORT_FORMAT_PO,
	}))
	if err != nil {
		t.Fatalf("ExportZip() error = %v", err)
	}

	files := unzip(t, res.Msg.Content)

	// Mail in its own PHP, narrowed to billing; Web, with no default, in the requested PO.
	if names := slices.Sorted(maps.Keys(files)); !slices.Equal(names, []string{"mail/ex.php", "manifest.json", "web-site/ex.po"}) {
		t.Fatalf("files = %v", names)
	}

	if mail := files["mail/ex.php"]; !strings.Contains(mail, "'mail.receipt'") || strings.Contains(mail, "mail.welcome") {
		t.Errorf("mail/ex.php = %s", mail)
	}

	if web := files["web-site/ex.po"]; !strings.Contains(web, `"web.home"`) || !strings.Contains(web, `"web.about"`) || strings.Contains(web, "mail.") {
		t.Errorf("web-site/ex.po = %s", web)
	}

	var manifest zipManifest
	if err := json.Unmarshal([]byte(files["manifest.json"]), &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}

	want := []zipScopeFolder{
		{Folder: "mail", Slug: "mail", Name: "Mail", Format: FormatPHP, Tags: []string{"billing"}},
		{Folder: "web-site", Slug: "../web site", Name: "Web", Format: FormatPO},
	}
	if manifest.Version != "2.0" || manifest.Layout != layoutScopes || manifest.Format != FormatPO ||
		!slices.Equal(manifest.Locales, []string{"ex"}) || !slices.EqualFunc(manifest.Scopes, want, sameFolder) {
		t.Errorf("manifest = %+v", manifest)
	}

	if res.Msg.Filename == "" || !strings.HasPrefix(res.Msg.Filename, "translations-Mail-Web-") {
		t.Errorf("filename = %s", res.Msg.Filename)
	}

	// A tag alone gives its scope a folder.
	res, err = server.ExportZip(ctx, connect.NewRequest(&pb.ExportZipRequest{TagIds: []string{f.home.ID}, LocaleIds: []string{f.ex.ID}}))
	if err != nil {
		t.Fatalf("ExportZip() error = %v", err)
	}

	if web := unzip(t, res.Msg.Content)["web-site/ex.json"]; !slices.Equal(jsonKeys(t, web), []string{"web.home"}) {
		t.Errorf("web-site/ex.json = %s", web)
	}
}

func sameFolder(a, b zipScopeFolder) bool {
	return a.Folder == b.Folder && a.Slug == b.Slug && a.Name == b.Name && a.Format == b.Format && slices.Equal(a.Tags, b.Tags)
}

func TestExportZipFlat(t *testing.T) {
	ctx, db := testutil.Setup(t)
	f := seedSelection(t, db)
	server := &Server{db: db}

	res, err := server.ExportZip(ctx, connect.NewRequest(&pb.ExportZipRequest{Format: pb.ExportFormat_EXPORT_FORMAT_PHP}))
	if err != nil {
		t.Fatalf("ExportZip() error = %v", err)
	}

	files := unzip(t, res.Msg.Content)
	if names := slices.Sorted(maps.Keys(files)); !slices.Equal(names, []string{"ex.php", "fr.php", "manifest.json"}) {
		t.Fatalf("files = %v", names)
	}

	if !strings.Contains(files["ex.php"], "'other'") || !strings.Contains(files["ex.php"], "'mail.welcome'") {
		t.Errorf("ex.php = %s", files["ex.php"])
	}

	var manifest zipManifest
	if err := json.Unmarshal([]byte(files["manifest.json"]), &manifest); err != nil || manifest.Layout != layoutFlat ||
		manifest.Format != FormatPHP || manifest.Scopes != nil || !slices.Equal(manifest.Locales, []string{"ex", "fr"}) {
		t.Errorf("manifest = %+v, %v", manifest, err)
	}

	testutil.ExpectInvalidArgument(t, ctx, server.ExportZip,
		&pb.ExportZipRequest{Format: pb.ExportFormat(99)},
		&pb.ExportZipRequest{LocaleIds: []string{"missing"}},
		&pb.ExportZipRequest{LocaleIds: []string{f.ex.ID, "missing"}},
		&pb.ExportZipRequest{TagIds: []string{"missing"}},
	)
}

func TestFolderName(t *testing.T) {
	taken := map[string]bool{}

	for _, tc := range []struct{ slug, want string }{
		{"backend", "backend"},
		{"../web site", "web-site"},
		{"..", "scope"},
		{".hidden", "hidden"},
		{"a/b\\c", "a-b-c"},
		{"Ünïcode_1.0", "n-code_1.0"},
		{"backend", "backend-2"},
		{"backend", "backend-3"},
		{"", "scope-2"},
	} {
		if got := folderName(tc.slug, taken); got != tc.want {
			t.Errorf("folderName(%q) = %q, want %q", tc.slug, got, tc.want)
		}
	}
}
