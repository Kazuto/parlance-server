package importservice

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
)

func (s *Server) ImportZip(
	ctx context.Context,
	req *connect.Request[pb.ImportZipRequest],
) (*connect.Response[pb.ImportZipResponse], error) {
	options, err := s.startImport(ctx, req.Msg.Mode, req.Msg.Approve, req.Msg.DryRun, req.Msg.FileContent)
	if err != nil {
		return nil, err
	}

	// Open ZIP archive
	zipReader, err := zip.NewReader(bytes.NewReader(req.Msg.FileContent), int64(len(req.Msg.FileContent)))
	if err != nil {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "invalid ZIP file")
	}

	response := &pb.ImportZipResponse{
		Results: make([]*pb.LocaleImportResult, 0),
	}

	// Each file is imported in its own transaction, the default locale's first. A dry run
	// imports them all in one, which it rolls back.
	err = s.runImport(ctx, options.dryRun, func(db *gorm.DB) error {
		defaultLocale, err := common.DefaultLocale(db)
		if err != nil {
			return err
		}

		// Process each file in the ZIP
		for _, file := range sourceFirst(zipReader.File, defaultLocale) {
			// Skip directories and manifest.json
			if file.FileInfo().IsDir() || strings.ToLower(file.Name) == "manifest.json" {
				continue
			}

			// Only process .json files
			ext := strings.ToLower(filepath.Ext(file.Name))
			if ext != ".json" {
				continue
			}

			localeCode := zipLocaleCode(file)

			// Find locale by code
			var locale models.Locale
			if err := db.Where("code = ?", localeCode).First(&locale).Error; err != nil {
				response.Results = append(response.Results, &pb.LocaleImportResult{
					LocaleCode: localeCode,
					Error:      fmt.Sprintf("locale not found: %s", localeCode),
				})

				continue
			}

			// Read file content
			reader, err := file.Open()
			if err != nil {
				response.Results = append(response.Results, &pb.LocaleImportResult{
					LocaleCode: localeCode,
					Error:      fmt.Sprintf("failed to open file: %v", err),
				})

				continue
			}

			buf := new(bytes.Buffer)
			if _, err := buf.ReadFrom(reader); err != nil {
				reader.Close()

				response.Results = append(response.Results, &pb.LocaleImportResult{
					LocaleCode: localeCode,
					Error:      fmt.Sprintf("failed to read file: %v", err),
				})

				continue
			}

			reader.Close()

			// Parse JSON content
			translations, err := parseJSONTranslations(buf.Bytes())
			if err != nil {
				response.Results = append(response.Results, &pb.LocaleImportResult{
					LocaleCode: localeCode,
					Error:      fmt.Sprintf("invalid JSON format: %v", err),
				})

				continue
			}

			// Process import for this locale
			result, err := importTranslations(ctx, db, locale.ID, req.Msg.ScopeIds, translations, options)
			if err != nil {
				response.Results = append(response.Results, &pb.LocaleImportResult{
					LocaleCode: localeCode,
					Error:      err.Error(),
				})

				continue
			}

			// Add successful result
			response.Results = append(response.Results, &pb.LocaleImportResult{
				LocaleCode: localeCode,
				Result:     result,
			})

			// Update totals
			response.TotalCreated += result.EntriesCreated
			response.TotalUpdated += result.EntriesUpdated
			response.TotalSkipped += result.EntriesSkipped
			response.TotalFailed += result.EntriesFailed
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(response.Results) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "no valid JSON files found in ZIP archive")
	}

	return connect.NewResponse(response), nil
}

// zipLocaleCode returns the code of the locale a file holds, e.g. "en" for "en.json"
func zipLocaleCode(file *zip.File) string {
	return strings.TrimSuffix(filepath.Base(file.Name), filepath.Ext(file.Name))
}

// sourceFirst puts the default locale's file first: the other files' translations of the
// entries whose default text it changes are then written for the new text, and stay
// current.
func sourceFirst(files []*zip.File, defaultLocale *models.Locale) []*zip.File {
	if defaultLocale == nil {
		return files
	}

	rank := func(file *zip.File) int {
		if zipLocaleCode(file) == defaultLocale.Code {
			return 0
		}

		return 1
	}

	ordered := slices.Clone(files)
	slices.SortStableFunc(ordered, func(a, b *zip.File) int {
		return cmp.Compare(rank(a), rank(b))
	})

	return ordered
}
