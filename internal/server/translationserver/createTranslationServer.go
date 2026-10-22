package translationserver

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/permissions"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// CreateTranslationServer adds a translation server
func (s *Server) CreateTranslationServer(
	ctx context.Context,
	req *connect.Request[pb.CreateTranslationServerRequest],
) (*connect.Response[pb.CreateTranslationServerResponse], error) {
	if err := middleware.RequirePermission(ctx, s.db, permissions.CreateTranslationServer); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "name is required")
	}
	baseURL := strings.TrimSpace(req.Msg.BaseUrl)
	if err := validateBaseURL(baseURL); err != nil {
		return nil, err
	}
	maxConcurrency := req.Msg.MaxConcurrency
	if maxConcurrency == 0 {
		maxConcurrency = 2
	}
	if err := validateMaxConcurrency(maxConcurrency); err != nil {
		return nil, err
	}
	apiKey, err := s.encryptAPIKey(req.Msg.ApiKey)
	if err != nil {
		return nil, err
	}

	server := &models.TranslationServer{
		Name:           name,
		BaseURL:        baseURL,
		Model:          strings.TrimSpace(req.Msg.Model),
		APIKey:         apiKey,
		MaxConcurrency: int(maxConcurrency),
		Enabled:        req.Msg.Enabled,
	}
	if userID, _ := ctx.Value(middleware.ContextKeyUserID).(string); userID != "" {
		server.CreatedBy = &userID
	}

	if err := s.db.Create(server).Error; err != nil {
		return nil, errors.Internal(errors.ErrInternal, "failed to create translation server")
	}

	return connect.NewResponse(&pb.CreateTranslationServerResponse{
		TranslationServer: converter.TranslationServerToProto(server),
	}), nil
}
