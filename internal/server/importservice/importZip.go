package importservice

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"path/filepath"
	"slices"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
)

func (s *Server) ImportZip(
	ctx context.Context,
	req *connect.Request[pb.ImportZipRequest],
) (*connect.Response[pb.ImportZipResponse], error) {
	options, err := s.startImport(ctx, req.Msg.Mode, req.Msg.Approve, req.Msg.DryRun, req.Msg.DeleteMissing, req.Msg.FileContent)
	if err != nil {
		return nil, err
	}

	// Open ZIP archive
	zipReader, err := zip.NewReader(bytes.NewReader(req.Msg.FileContent), int64(len(req.Msg.FileContent)))
	if err != nil {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "invalid ZIP file")
	}

	// Creating languages needs the permission, which also lets a dry run show the
	// files of missing languages as if they were created.
	if len(req.Msg.CreateLocaleCodes) > 0 {
		if err := middleware.RequirePermission(ctx, s.db, permissions.CreateLocale); err != nil {
			return nil, err
		}
	}

	canCreate := middleware.HasPermission(ctx, s.db, permissions.CreateLocale)

	response := &pb.ImportZipResponse{
		Results: make([]*pb.LocaleImportResult, 0),
	}

	// Each file is imported in its own transaction, the default locale's first, and then
	// delete_missing deletes in one more. A dry run does all in one, which it rolls back.
	err = s.runImport(ctx, options.dryRun, func(db *gorm.DB) error {
		defaultLocale, err := common.DefaultLocale(db)
		if err != nil {
			return err
		}

		// delete_missing goes by the default language's files, once all files are in
		var held map[string]bool

		if options.deleteMissing {
			if held, err = zipHeldKeys(zipReader.File, defaultLocale); err != nil {
				return err
			}
		}

		fileOptions := options
		fileOptions.deleteMissing = false

		archive := newZipImport(ctx, common.GetLocaleFromRequest(req), req.Msg.ScopeIds, fileOptions, canCreate, req.Msg.CreateLocaleCodes)
		archive.held = held

		for _, file := range sourceFirst(zipReader.File, defaultLocale) {
			if !isZipLanguageFile(file) {
				continue
			}

			result, err := archive.importFile(db, file)
			if err != nil {
				return err
			}

			response.Results = append(response.Results, result)

			if result.Result != nil {
				response.TotalCreated += result.Result.EntriesCreated
				response.TotalUpdated += result.Result.EntriesUpdated
				response.TotalSkipped += result.Result.EntriesSkipped
				response.TotalFailed += result.Result.EntriesFailed
			}
		}

		if !options.deleteMissing {
			return nil
		}

		err = db.Transaction(func(tx *gorm.DB) (err error) {
			response.Deletion, err = deleteMissingEntries(tx, middleware.UserID(ctx), req.Msg.ScopeIds, held, options.dryRun)

			return err
		})
		if err != nil && connect.CodeOf(err) == connect.CodeUnknown {
			return errors.InternalCause(err, "failed to delete the missing entries")
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	if len(response.Results) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "no valid JSON files found in ZIP archive")
	}

	return connect.NewResponse(response), nil
}

// isZipLanguageFile tells the archive's language files from folders, manifest.json and
// other files
func isZipLanguageFile(file *zip.File) bool {
	return !file.FileInfo().IsDir() && !strings.EqualFold(file.Name, "manifest.json") &&
		strings.EqualFold(filepath.Ext(file.Name), ".json")
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
		if strings.EqualFold(zipLocaleCode(file), defaultLocale.Code) {
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
