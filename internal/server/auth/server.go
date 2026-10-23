package auth

import (
	"time"

	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	authpkg "github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/sso"
)

// Server implements the AuthService
type Server struct {
	db     *database.DB
	config *config.Config
	// sso signs users in with single sign-on providers.
	sso *sso.Client
}

// NewServer creates a new AuthService server
func NewServer(db *database.DB, cfg *config.Config, ssoClient *sso.Client) parlancev1connect.AuthServiceHandler {
	return &Server{
		db:     db,
		config: cfg,
		sso:    ssoClient,
	}
}

// issueTokens creates an access and a refresh token for the user. The refresh token lasts
// seven times as long as the configured access token expiry, which defaults to 24 hours.
func (s *Server) issueTokens(user *models.User) (*authpkg.TokenPair, error) {
	accessExpiry, err := time.ParseDuration(s.config.JWT.Expiry)
	if err != nil {
		accessExpiry = 24 * time.Hour
	}

	tokens, err := authpkg.GenerateTokenPair(user.ID, user.Email, s.config.JWT.Secret, accessExpiry, accessExpiry*7)
	if err != nil {
		return nil, errors.InternalCause(err, "failed to generate tokens")
	}

	return tokens, nil
}
