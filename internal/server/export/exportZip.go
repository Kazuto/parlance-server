package export

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
)

// ZIP layouts: files named "<locale>.<ext>", or a folder per scope, "<folder>/<locale>.<ext>"
const (
	layoutFlat   = "flat"
	layoutScopes = "scopes"
)

// unsafeFolderChars matches what a folder name in a ZIP export can't hold
var unsafeFolderChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// zipManifest describes a ZIP export in its manifest.json
type zipManifest struct {
	ExportedAt string `json:"exported_at"`
	Version    string `json:"version"`
	Layout     string `json:"layout"`
	// Format is the flat files' format, or that of scopes without a default.
	Format  string           `json:"format"`
	Locales []string         `json:"locales"`
	Scopes  []zipScopeFolder `json:"scopes,omitempty"`
}

// zipScopeFolder is a scope's folder in a ZIP export
type zipScopeFolder struct {
	Folder string   `json:"folder"`
	Slug   string   `json:"slug"`
	Name   string   `json:"name"`
	Format string   `json:"format"`
	Tags   []string `json:"tags,omitempty"`

	selection Selection
}

func (s *Server) ExportZip(
	ctx context.Context,
	req *connect.Request[pb.ExportZipRequest],
) (*connect.Response[pb.ExportZipResponse], error) {
	db := s.db.WithContext(ctx)

	sel, err := selection(db, req.Msg)
	if err != nil {
		return nil, err
	}

	format := FormatJSON
	if req.Msg.Format != pb.ExportFormat_EXPORT_FORMAT_UNSPECIFIED {
		if format = converter.ExportFormatFromProto(req.Msg.Format); format == "" {
			return nil, errors.InvalidInput(errors.ErrInvalidInput, "unknown export format")
		}
	}

	locales, err := zipLocales(db, req.Msg.LocaleIds)
	if err != nil {
		return nil, err
	}

	folders, err := scopeFolders(db, sel, format)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	archive := zip.NewWriter(buf)
	manifest := zipManifest{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Version:    "2.0",
		Layout:     layoutFlat,
		Format:     format,
		Locales:    make([]string, 0, len(locales)),
		Scopes:     folders,
	}

	if len(folders) > 0 {
		manifest.Layout = layoutScopes
	}

	for _, locale := range locales {
		if len(folders) == 0 {
			if err := writeLocaleFile(db, archive, "", format, locale, sel); err != nil {
				return nil, err
			}
		}

		for _, folder := range folders {
			if err := writeLocaleFile(db, archive, folder.Folder+"/", folder.Format, locale, folder.selection); err != nil {
				return nil, err
			}
		}

		manifest.Locales = append(manifest.Locales, locale.Code)
	}

	manifestBytes, err := marshalJSON(manifest)
	if err != nil {
		return nil, errors.InternalCause(err, "failed to marshal manifest")
	}

	if err := writeZipFile(archive, "manifest.json", manifestBytes); err != nil {
		return nil, err
	}

	if err := archive.Close(); err != nil {
		return nil, errors.InternalCause(err, "failed to close ZIP archive")
	}

	// e.g. "translations-backend-shared-2006-01-02.zip"
	filename := "translations-"
	if names := scopeNames(db, sel.ScopeIDs); names != "" {
		filename += names + "-"
	}

	filename += time.Now().Format("2006-01-02") + ".zip"

	return connect.NewResponse(&pb.ExportZipResponse{
		Format:   "zip",
		Content:  buf.Bytes(),
		Filename: filename,
	}), nil
}

// zipLocales loads the locales with the IDs, or all without IDs, ordered by code
func zipLocales(db *gorm.DB, ids []string) ([]models.Locale, error) {
	if ids = common.UniqueIDs(ids); len(ids) > 0 {
		return common.ResolveByIDs[models.Locale](db.Order("code"), ids, errors.ErrLocaleNotFound, "one or more languages do not exist")
	}

	var locales []models.Locale
	if err := db.Order("code").Find(&locales).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to query locales")
	}

	if len(locales) == 0 {
		return nil, errors.NotFound(errors.ErrLocaleNotFound, "no locales found")
	}

	return locales, nil
}

// scopeFolders gives each selected scope, and each scope of a selected tag, a folder of
// files in its default format (or the fallback), ordered by name. Without a selection
// there are none.
func scopeFolders(db *gorm.DB, sel Selection, fallback string) ([]zipScopeFolder, error) {
	var tags []models.Tag
	if len(sel.TagIDs) > 0 {
		if err := db.Where("id IN ?", sel.TagIDs).Order("name").Find(&tags).Error; err != nil {
			return nil, errors.InternalCause(err, "failed to load tags")
		}
	}

	scopeIDs := sel.ScopeIDs
	for _, tag := range tags {
		scopeIDs = append(scopeIDs, tag.ScopeID)
	}

	if scopeIDs = common.UniqueIDs(scopeIDs); len(scopeIDs) == 0 {
		return nil, nil
	}

	var scopes []models.Scope
	if err := db.Where("id IN ?", scopeIDs).Order("name").Find(&scopes).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load scopes")
	}

	folders := make([]zipScopeFolder, len(scopes))
	taken := map[string]bool{}

	for i, scope := range scopes {
		folder := zipScopeFolder{
			Folder:    folderName(scope.Slug, taken),
			Slug:      scope.Slug,
			Name:      scope.Name,
			Format:    cmp.Or(scope.ExportFormat, fallback),
			selection: Selection{ScopeIDs: []string{scope.ID}, IncludeDrafts: sel.IncludeDrafts, IncludeUnreleased: sel.IncludeUnreleased},
		}

		// Only the scope's own tags, so another scope's tags don't pull their entries in.
		for _, tag := range tags {
			if tag.ScopeID == scope.ID {
				folder.Tags = append(folder.Tags, tag.Name)
				folder.selection.TagIDs = append(folder.selection.TagIDs, tag.ID)
			}
		}

		folders[i] = folder
	}

	return folders, nil
}

// folderName makes a scope's slug a folder name of letters, digits and . _ -, which
// never is "." or "..", and numbers it when another scope's folder has the name.
func folderName(slug string, taken map[string]bool) string {
	base := strings.Trim(unsafeFolderChars.ReplaceAllString(slug, "-"), "-.")
	if base == "" {
		base = "scope"
	}

	name := base
	for n := 2; taken[name]; n++ {
		name = base + "-" + strconv.Itoa(n)
	}

	taken[name] = true

	return name
}

// writeLocaleFile renders the locale's file for the selection into the archive's folder
func writeLocaleFile(db *gorm.DB, archive *zip.Writer, folder, format string, locale models.Locale, sel Selection) error {
	content, err := Render(db, format, locale, sel)
	if err != nil {
		return err
	}

	return writeZipFile(archive, folder+locale.Code+"."+format, []byte(content))
}

// writeZipFile adds a file to the archive
func writeZipFile(archive *zip.Writer, name string, content []byte) error {
	writer, err := archive.Create(name)
	if err != nil {
		return errors.InternalCause(err, "failed to create ZIP entry for "+name)
	}

	if _, err := writer.Write(content); err != nil {
		return errors.InternalCause(err, "failed to write ZIP entry for "+name)
	}

	return nil
}
