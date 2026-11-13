package importservice

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/langnames"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// zipImport imports the files of a ZIP archive one by one.
type zipImport struct {
	ctx      context.Context
	scopeIDs []string
	options  importOptions
	// canCreate tells whether the user may create languages; create holds the codes
	// of the missing ones to create.
	canCreate bool
	create    []string
	// language is the request's, which results name missing languages in; languages
	// are those new languages get names in.
	language  string
	languages []string
	// created holds the languages created for files so far, by lower-case code.
	created map[string]bool
}

// newZipImport starts a ZIP import. New languages get names in English, the
// request's language and the project's languages.
func newZipImport(
	ctx context.Context, db *gorm.DB, language string, scopeIDs []string, options importOptions, canCreate bool, create []string,
) (*zipImport, error) {
	var codes []string
	if err := db.Model(&models.Locale{}).Order("code").Pluck("code", &codes).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to load locales")
	}

	languages := append([]string{"en", language}, codes...)
	slices.Sort(languages)

	return &zipImport{
		ctx: ctx, scopeIDs: scopeIDs, options: options, canCreate: canCreate, create: create,
		language: language, languages: slices.Compact(languages), created: map[string]bool{},
	}, nil
}

// importFile imports a language file of the archive. What goes wrong with the file
// is in the result; the error is the server's own.
func (z *zipImport) importFile(db *gorm.DB, file *zip.File) (*pb.LocaleImportResult, error) {
	code := zipLocaleCode(file)
	result := &pb.LocaleImportResult{LocaleCode: code}

	translations, err := readZipJSON(file)
	if err != nil {
		result.Error = err.Error()

		return result, nil
	}

	locale, reason, err := resolveZipLocale(db, code, z.canCreate)
	if err != nil {
		return nil, err
	}

	if locale != nil {
		result.LocaleCode = locale.Code
		if z.created[strings.ToLower(locale.Code)] {
			z.markNew(result)
		}

		z.importInto(db, locale.ID, translations, result)

		return result, nil
	}

	result.LocaleMissing = true
	result.LocaleName = z.name(code)
	result.MissingReason = reason

	switch {
	case reason == pb.MissingLocaleReason_MISSING_LOCALE_REASON_DELETED:
		result.Error = fmt.Sprintf("language %s was deleted; restore it under Locales to import its file", code)
	case reason == pb.MissingLocaleReason_MISSING_LOCALE_REASON_INVALID_CODE:
		result.Error = fmt.Sprintf("%s is no known language code; add the language under Locales to import its file", code)
	case reason == pb.MissingLocaleReason_MISSING_LOCALE_REASON_NO_PERMISSION:
		result.Error = fmt.Sprintf("language %s doesn't exist, and creating it needs the create_locale permission; file skipped", code)
	case !z.options.dryRun && !slices.ContainsFunc(z.create, func(listed string) bool { return strings.EqualFold(listed, code) }):
		result.Error = fmt.Sprintf("language %s doesn't exist; file skipped", code)
	default:
		// A dry run creates every language it can, so it shows what their files change.
		z.createAndImport(db, code, translations, result)
	}

	return result, nil
}

// createAndImport creates the file's language and imports the file into it, both or
// neither.
func (z *zipImport) createAndImport(db *gorm.DB, code string, translations map[string]string, result *pb.LocaleImportResult) {
	err := db.Transaction(func(tx *gorm.DB) error {
		locale, err := createZipLocale(tx, code, z.languages)
		if err != nil {
			return err
		}

		imported, err := importTranslations(z.ctx, tx, locale.ID, z.scopeIDs, translations, z.options)
		if err != nil {
			return err
		}

		result.Result = imported

		return nil
	})
	if err != nil {
		result.Result = nil
		result.Error = err.Error()

		return
	}

	z.created[strings.ToLower(code)] = true
	z.markNew(result)
}

// importInto imports the file into an existing language.
func (z *zipImport) importInto(db *gorm.DB, localeID string, translations map[string]string, result *pb.LocaleImportResult) {
	imported, err := importTranslations(z.ctx, db, localeID, z.scopeIDs, translations, z.options)
	if err != nil {
		result.Error = err.Error()

		return
	}

	result.Result = imported
}

// markNew marks the result of a file whose language the import creates.
func (z *zipImport) markNew(result *pb.LocaleImportResult) {
	result.LocaleName = z.name(result.LocaleCode)
	result.LocaleMissing = z.options.dryRun
	result.LocaleCreated = !z.options.dryRun
}

// name names a language in the request's language, else in English, else by its code.
func (z *zipImport) name(code string) string {
	return cmp.Or(langnames.Name(code, z.language), langnames.Name(code, "en"), code)
}

// readZipJSON reads the translations of a language file.
func readZipJSON(file *zip.File) (map[string]string, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	translations, err := parseJSONTranslations(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	return translations, nil
}

// resolveZipLocale finds the language with the code in any case, so "de-de.json"
// goes into "de-DE", preferring the code as it is. Without one, the reason says
// why it can't be created, unspecified when it can.
func resolveZipLocale(db *gorm.DB, code string, canCreate bool) (*models.Locale, pb.MissingLocaleReason, error) {
	var locales []models.Locale
	if err := db.Where("LOWER(code) = LOWER(?)", code).Order("code").Find(&locales).Error; err != nil {
		return nil, 0, errors.InternalCause(err, "failed to load locale")
	}

	if len(locales) > 0 {
		exact := slices.IndexFunc(locales, func(locale models.Locale) bool { return locale.Code == code })

		return &locales[max(exact, 0)], pb.MissingLocaleReason_MISSING_LOCALE_REASON_UNSPECIFIED, nil
	}

	deleted, err := common.Exists(db.Unscoped().Model(&models.Locale{}).Where("LOWER(code) = LOWER(?)", code))
	if err != nil {
		return nil, 0, errors.InternalCause(err, "failed to check deleted locales")
	}

	switch {
	case deleted:
		return nil, pb.MissingLocaleReason_MISSING_LOCALE_REASON_DELETED, nil
	case !langnames.Valid(code):
		return nil, pb.MissingLocaleReason_MISSING_LOCALE_REASON_INVALID_CODE, nil
	case !canCreate:
		return nil, pb.MissingLocaleReason_MISSING_LOCALE_REASON_NO_PERMISSION, nil
	}

	return nil, pb.MissingLocaleReason_MISSING_LOCALE_REASON_UNSPECIFIED, nil
}

// createZipLocale creates the language of a file, named in the languages as far as
// they are known, and in English at least by its code. It never becomes the
// default.
func createZipLocale(tx *gorm.DB, code string, languages []string) (*models.Locale, error) {
	names := langnames.Names(code, languages)
	names["en"] = cmp.Or(names["en"], code)

	encoded, err := json.Marshal(names)
	if err != nil {
		return nil, errors.InternalCause(err, "failed to encode names")
	}

	locale := models.Locale{Code: code, Names: datatypes.JSON(encoded)}
	if err := tx.Create(&locale).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to create language "+code)
	}

	return &locale, nil
}
