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

// ScopeServer implements the ScopeService
type ScopeServer struct {
	db *database.DB
}

// NewScopeServer creates a new ScopeServer
func NewScopeServer(db *database.DB) parlancev1connect.ScopeServiceHandler {
	return &ScopeServer{db: db}
}

// ListScopes returns a paginated list of scopes
func (s *ScopeServer) ListScopes(
	ctx context.Context,
	req *connect.Request[pb.ListScopesRequest],
) (*connect.Response[pb.ListScopesResponse], error) {
	page, perPage := converter.GetPaginationParams(req.Msg.Pagination)

	var scopes []models.Scope
	var total int64

	query := s.db.Model(&models.Scope{})

	if err := query.Count(&total).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count scopes: %w", err))
	}

	offset := (page - 1) * perPage
	if err := query.Offset(int(offset)).Limit(int(perPage)).Order("name ASC").Find(&scopes).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch scopes: %w", err))
	}

	return connect.NewResponse(&pb.ListScopesResponse{
		Scopes:     converter.ScopesToProto(scopes),
		Pagination: converter.PaginationToProto(total, page, perPage),
	}), nil
}

// GetScope returns a single scope by ID
func (s *ScopeServer) GetScope(
	ctx context.Context,
	req *connect.Request[pb.GetScopeRequest],
) (*connect.Response[pb.GetScopeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope id is required"))
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scope not found"))
	}

	return connect.NewResponse(&pb.GetScopeResponse{
		Scope: converter.ScopeToProto(&scope),
	}), nil
}

// CreateScope creates a new scope
func (s *ScopeServer) CreateScope(
	ctx context.Context,
	req *connect.Request[pb.CreateScopeRequest],
) (*connect.Response[pb.CreateScopeResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope name is required"))
	}

	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope slug is required"))
	}

	scope := &models.Scope{
		Name:        req.Msg.Name,
		Slug:        req.Msg.Slug,
		Description: req.Msg.Description,
		Color:       req.Msg.Color,
	}

	if err := s.db.Create(scope).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create scope: %w", err))
	}

	return connect.NewResponse(&pb.CreateScopeResponse{
		Scope: converter.ScopeToProto(scope),
	}), nil
}

// UpdateScope updates an existing scope
func (s *ScopeServer) UpdateScope(
	ctx context.Context,
	req *connect.Request[pb.UpdateScopeRequest],
) (*connect.Response[pb.UpdateScopeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope id is required"))
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scope not found"))
	}

	if req.Msg.Name != "" {
		scope.Name = req.Msg.Name
	}

	if req.Msg.Slug != "" {
		scope.Slug = req.Msg.Slug
	}

	if req.Msg.Description != "" {
		scope.Description = req.Msg.Description
	}

	if req.Msg.Color != "" {
		scope.Color = req.Msg.Color
	}

	if err := s.db.Save(&scope).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update scope: %w", err))
	}

	return connect.NewResponse(&pb.UpdateScopeResponse{
		Scope: converter.ScopeToProto(&scope),
	}), nil
}

// DeleteScope soft deletes a scope
func (s *ScopeServer) DeleteScope(
	ctx context.Context,
	req *connect.Request[pb.DeleteScopeRequest],
) (*connect.Response[pb.DeleteScopeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope id is required"))
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.Id).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scope not found"))
	}

	if err := s.db.Delete(&scope).Error; err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete scope: %w", err))
	}

	return connect.NewResponse(&pb.DeleteScopeResponse{}), nil
}

// AddScopeToEntry adds a scope to an entry
func (s *ScopeServer) AddScopeToEntry(
	ctx context.Context,
	req *connect.Request[pb.AddScopeToEntryRequest],
) (*connect.Response[pb.AddScopeToEntryResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	if req.Msg.ScopeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope id is required"))
	}

	var entry models.Entry
	if err := s.db.First(&entry, "id = ?", req.Msg.EntryId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("entry not found"))
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.ScopeId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scope not found"))
	}

	if err := s.db.Model(&entry).Association("Scopes").Append(&scope); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to add scope to entry: %w", err))
	}

	return connect.NewResponse(&pb.AddScopeToEntryResponse{}), nil
}

// RemoveScopeFromEntry removes a scope from an entry
func (s *ScopeServer) RemoveScopeFromEntry(
	ctx context.Context,
	req *connect.Request[pb.RemoveScopeFromEntryRequest],
) (*connect.Response[pb.RemoveScopeFromEntryResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	if req.Msg.ScopeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope id is required"))
	}

	var entry models.Entry
	if err := s.db.First(&entry, "id = ?", req.Msg.EntryId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("entry not found"))
	}

	var scope models.Scope
	if err := s.db.First(&scope, "id = ?", req.Msg.ScopeId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("scope not found"))
	}

	if err := s.db.Model(&entry).Association("Scopes").Delete(&scope); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to remove scope from entry: %w", err))
	}

	return connect.NewResponse(&pb.RemoveScopeFromEntryResponse{}), nil
}

// GetEntryScopes returns all scopes for an entry
func (s *ScopeServer) GetEntryScopes(
	ctx context.Context,
	req *connect.Request[pb.GetEntryScopesRequest],
) (*connect.Response[pb.GetEntryScopesResponse], error) {
	if req.Msg.EntryId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("entry id is required"))
	}

	var entry models.Entry
	if err := s.db.Preload("Scopes").First(&entry, "id = ?", req.Msg.EntryId).Error; err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("entry not found"))
	}

	return connect.NewResponse(&pb.GetEntryScopesResponse{
		Scopes: converter.ScopesToProto(entry.Scopes),
	}), nil
}
