package auth

import (
	"context"
	"net/url"
	"slices"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// ListSignInProviders returns the enabled single sign-on providers, by name
func (s *Server) ListSignInProviders(
	ctx context.Context,
	req *connect.Request[pb.ListSignInProvidersRequest],
) (*connect.Response[pb.ListSignInProvidersResponse], error) {
	var providers []models.SSOProvider
	if err := s.db.WithContext(ctx).Select("id", "name").Where("enabled = ?", true).Order("name").Find(&providers).Error; err != nil {
		return nil, errors.InternalCause(err, "failed to list sign-in providers")
	}

	result := make([]*pb.SignInProvider, len(providers))
	for i, provider := range providers {
		result[i] = &pb.SignInProvider{Id: provider.ID, Name: provider.Name}
	}

	return connect.NewResponse(&pb.ListSignInProvidersResponse{Providers: result}), nil
}

// StartSso begins signing in with a provider and returns where to send the browser
func (s *Server) StartSso(
	ctx context.Context,
	req *connect.Request[pb.StartSsoRequest],
) (*connect.Response[pb.StartSsoResponse], error) {
	if req.Msg.ProviderId == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "provider id is required")
	}

	if err := s.checkRedirectURI(req.Msg.RedirectUri); err != nil {
		return nil, err
	}

	authorizationURL, err := s.sso.Start(ctx, req.Msg.ProviderId, req.Msg.RedirectUri)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.StartSsoResponse{AuthorizationUrl: authorizationURL}), nil
}

// FinishSso completes signing in with what the provider sent back, like Login
func (s *Server) FinishSso(
	ctx context.Context,
	req *connect.Request[pb.FinishSsoRequest],
) (*connect.Response[pb.FinishSsoResponse], error) {
	if req.Msg.State == "" || req.Msg.Code == "" {
		return nil, errors.InvalidInput(errors.ErrInvalidInput, "state and code are required")
	}

	user, err := s.sso.Finish(ctx, req.Msg.State, req.Msg.Code)
	if err != nil {
		return nil, err
	}

	tokens, err := s.issueTokens(user)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&pb.FinishSsoResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		User:         converter.UserToProto(user),
	}), nil
}

// checkRedirectURI accepts an absolute http or https URL of a web app the API
// answers, so a sign-in can't be sent elsewhere. Providers check it again against
// the addresses registered with them.
func (s *Server) checkRedirectURI(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.InvalidInput(errors.ErrInvalidInput, "redirect uri must be an http or https URL")
	}

	origins := s.config.CORS.AllowedOrigins
	if slices.Contains(origins, "*") || slices.Contains(origins, parsed.Scheme+"://"+parsed.Host) {
		return nil
	}

	return errors.InvalidInput(errors.ErrInvalidInput, "redirect uri must belong to a web app in CORS_ALLOWED_ORIGINS")
}
