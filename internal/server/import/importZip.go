package importservice

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
)

func (s *Server) ImportZip(
	ctx context.Context,
	req *connect.Request[pb.ImportZipRequest],
) (*connect.Response[pb.ImportZipResponse], error) {
	if len(req.Msg.FileContent) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "file_content is required")
	}

	// Set default mode if not specified
	mode := req.Msg.Mode
	if mode == pb.ImportMode_IMPORT_MODE_UNSPECIFIED {
		mode = pb.ImportMode_IMPORT_MODE_CREATE_OR_UPDATE
	}

	// Open ZIP archive
	zipReader, err := zip.NewReader(bytes.NewReader(req.Msg.FileContent), int64(len(req.Msg.FileContent)))
	if err != nil {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "invalid ZIP file")
	}

	response := &pb.ImportZipResponse{
		Results: make([]*pb.LocaleImportResult, 0),
	}

	// Process each file in the ZIP
	for _, file := range zipReader.File {
		// Skip directories and manifest.json
		if file.FileInfo().IsDir() || strings.ToLower(file.Name) == "manifest.json" {
			continue
		}

		// Only process .json files
		ext := strings.ToLower(filepath.Ext(file.Name))
		if ext != ".json" {
			continue
		}

		// Extract locale code from filename (e.g., "en.json" -> "en")
		localeCode := strings.TrimSuffix(filepath.Base(file.Name), filepath.Ext(file.Name))

		// Find locale by code
		var locale models.Locale
		if err := s.db.Where("code = ?", localeCode).First(&locale).Error; err != nil {
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
		result, err := s.processImport(ctx, locale.ID, req.Msg.ScopeIds, translations, mode)
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

	if len(response.Results) == 0 {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "no valid JSON files found in ZIP archive")
	}

	return connect.NewResponse(response), nil
}
