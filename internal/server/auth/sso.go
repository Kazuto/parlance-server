package auth

import (
	"context"
	"net/url"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/sso"
	"github.com/kazuto/parlance-server/internal/webhook"

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

	user, attempt, err := s.sso.FinishAttempt(ctx, req.Msg.State, req.Msg.Code)
	if err != nil {
		s.reportSsoFailure(ctx, req, attempt, err)

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

// reportSsoFailure reports a single sign-on that failed. An expired sign-in is
// mostly a reload or the back button, and the server's own errors are no failed
// sign-in, so neither is reported.
func (s *Server) reportSsoFailure(ctx context.Context, req connect.AnyRequest, attempt sso.Attempt, err error) {
	code := errors.CodeOf(err)
	if code == "" || code == errors.ErrSSOLoginExpired || connect.CodeOf(err) == connect.CodeInternal {
		return
	}

	data := webhook.LoginFailedData{
		Email:  attempt.Email,
		Method: webhook.LoginMethodSSO,
		Reason: strings.ToLower(code),
	}

	if attempt.ProviderID != "" {
		data.Provider = &webhook.ProviderInfo{ID: attempt.ProviderID, Name: attempt.ProviderName}
	}

	s.failures.report(ctx, req.Peer().Addr, req.Header(), data)
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
