package server

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
)

// LocaleServer implements the LocaleService
type LocaleServer struct {
	db *database.DB
}

// NewLocaleServer creates a new LocaleServer
func NewLocaleServer(db *database.DB) parlancev1connect.LocaleServiceHandler {
	return &LocaleServer{db: db}
}

// getLocaleFromRequest extracts locale preference from request headers
func getLocaleFromRequest[T any](req *connect.Request[T]) string {
	acceptLang := req.Header().Get("Accept-Language")

	if acceptLang != "" && len(acceptLang) >= 2 {
		return acceptLang[:2]
	}

	return "en"
}

// ListLocales returns a paginated list of locales
func (s *LocaleServer) ListLocales(
	ctx context.Context,
	req *connect.Request[pb.ListLocalesRequest],
) (*connect.Response[pb.ListLocalesResponse], error) {
	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)
	requestLocale := getLocaleFromRequest(req)

	var locales []models.Locale
	var total int64

	query := s.db.Model(&models.Locale{})

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count locales: %w", err))
	}

	// Get paginated results
	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Order("code ASC").Find(&locales).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch locales: %w", err))
	}

	return connect.NewResponse(&pb.ListLocalesResponse{
		Locales:    converter.LocalesToProto(locales, requestLocale),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}

// GetLocale returns a single locale by ID
func (s *LocaleServer) GetLocale(
	ctx context.Context,
	req *connect.Request[pb.GetLocaleRequest],
) (*connect.Response[pb.GetLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	requestLocale := getLocaleFromRequest(req)

	return connect.NewResponse(&pb.GetLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}

// CreateLocale creates a new locale
func (s *LocaleServer) CreateLocale(
	ctx context.Context,
	req *connect.Request[pb.CreateLocaleRequest],
) (*connect.Response[pb.CreateLocaleResponse], error) {
	if req.Msg.Code == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale code is required"))
	}

	if req.Msg.Names == nil || len(req.Msg.Names) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale names are required"))
	}

	names, err := converter.NamesMapToJSON(req.Msg.Names)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to encode names: %w", err))
	}

	// If setting as default, unset other defaults
	if req.Msg.IsDefault {
		if err := s.db.Model(&models.Locale{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update default locale: %w", err))
		}
	}

	locale := &models.Locale{
		Code:      req.Msg.Code,
		Names:     names,
		IsDefault: req.Msg.IsDefault,
	}

	if err := s.db.Create(locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create locale: %w", err))
	}

	requestLocale := getLocaleFromRequest(req)

	return connect.NewResponse(&pb.CreateLocaleResponse{
		Locale: converter.LocaleToProto(locale, requestLocale),
	}), nil
}

// UpdateLocale updates an existing locale
func (s *LocaleServer) UpdateLocale(
	ctx context.Context,
	req *connect.Request[pb.UpdateLocaleRequest],
) (*connect.Response[pb.UpdateLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	// If setting as default, unset other defaults
	if req.Msg.IsDefault && !locale.IsDefault {
		if err := s.db.Model(&models.Locale{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update default locale: %w", err))
		}
	}

	// Update fields
	if req.Msg.Code != "" {
		locale.Code = req.Msg.Code
	}

	if req.Msg.Names != nil && len(req.Msg.Names) > 0 {
		names, err := converter.NamesMapToJSON(req.Msg.Names)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to encode names: %w", err))
		}

		locale.Names = names
	}

	locale.IsDefault = req.Msg.IsDefault

	if err := s.db.Save(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update locale: %w", err))
	}

	requestLocale := getLocaleFromRequest(req)

	return connect.NewResponse(&pb.UpdateLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}

// DeleteLocale soft deletes a locale
func (s *LocaleServer) DeleteLocale(
	ctx context.Context,
	req *connect.Request[pb.DeleteLocaleRequest],
) (*connect.Response[pb.DeleteLocaleResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	// Prevent deleting the default locale
	if locale.IsDefault {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("cannot delete the default locale"))
	}

	if err := s.db.Delete(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete locale: %w", err))
	}

	return connect.NewResponse(&pb.DeleteLocaleResponse{}), nil
}

// GetDefaultLocale returns the default locale
func (s *LocaleServer) GetDefaultLocale(
	ctx context.Context,
	req *connect.Request[pb.GetDefaultLocaleRequest],
) (*connect.Response[pb.GetDefaultLocaleResponse], error) {
	var locale models.Locale
	if err := s.db.Where("is_default = ?", true).First(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("default locale not found"))
	}

	requestLocale := getLocaleFromRequest(req)

	return connect.NewResponse(&pb.GetDefaultLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}

// SetDefaultLocale sets a locale as the default
func (s *LocaleServer) SetDefaultLocale(
	ctx context.Context,
	req *connect.Request[pb.SetDefaultLocaleRequest],
) (*connect.Response[pb.SetDefaultLocaleResponse], error) {
	if req.Msg.LocaleId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("locale id is required"))
	}

	var locale models.Locale
	if err := s.db.First(&locale, "id = ?", req.Msg.LocaleId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("locale not found"))
	}

	// Unset other defaults
	if err := s.db.Model(&models.Locale{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update default locale: %w", err))
	}

	// Set this as default
	locale.IsDefault = true
	if err := s.db.Save(&locale).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to set default locale: %w", err))
	}

	requestLocale := getLocaleFromRequest(req)

	return connect.NewResponse(&pb.SetDefaultLocaleResponse{
		Locale: converter.LocaleToProto(&locale, requestLocale),
	}), nil
}
