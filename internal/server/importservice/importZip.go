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
	options, err := s.startImport(ctx, req.Msg.Mode, req.Msg.Approve, req.Msg.DryRun, req.Msg.FileContent)
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

	// Each file is imported in its own transaction, the default locale's first. A dry run
	// imports them all in one, which it rolls back.
	err = s.runImport(ctx, options.dryRun, func(db *gorm.DB) error {
		defaultLocale, err := common.DefaultLocale(db)
		if err != nil {
			return err
		}

		archive, err := newZipImport(ctx, db, common.GetLocaleFromRequest(req), req.Msg.ScopeIds, options, canCreate, req.Msg.CreateLocaleCodes)
		if err != nil {
			return err
		}

		for _, file := range sourceFirst(zipReader.File, defaultLocale) {
			// Only language files: no folders, manifest.json or other files
			if file.FileInfo().IsDir() || strings.EqualFold(file.Name, "manifest.json") ||
				!strings.EqualFold(filepath.Ext(file.Name), ".json") {
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
