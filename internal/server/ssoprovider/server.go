// Package ssoprovider serves the SsoProviderService. Package sso signs users in
// with the providers.
package ssoprovider

import (
	"context"
	"net/url"
	"strings"

	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/server/common"
	"gorm.io/gorm"
)

// maxNameLength caps a provider's name, which its sign-in button shows.
const maxNameLength = 100

// Server implements the SsoProviderService
type Server struct {
	db  *database.DB
	box *secret.Box
	// check fetches an issuer's configuration for TestSsoProvider.
	check func(ctx context.Context, issuerURL string) error
}

// NewServer creates a new SsoProviderService server. check fetches an issuer's
// OpenID configuration.
func NewServer(db *database.DB, box *secret.Box, check func(ctx context.Context, issuerURL string) error) parlancev1connect.SsoProviderServiceHandler {
	return &Server{db: db, box: box, check: check}
}

func (s *Server) findProvider(ctx context.Context, id string) (models.SSOProvider, error) {
	if id == "" {
		return models.SSOProvider{}, errors.InvalidInput(errors.ErrInvalidInput, "provider id is required")
	}

	return common.FindByID[models.SSOProvider](s.db.WithContext(ctx), id, errors.ErrSSOProviderNotFound, "sign-in provider not found")
}

// encryptSecret returns the client secret to store: encrypted, or empty for none.
func (s *Server) encryptSecret(plain string) (string, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return "", nil
	}

	encrypted, err := s.box.Encrypt(plain)
	if err != nil {
		return "", errors.FailedPrecondition(errors.ErrEncryptionKeyMissing, "set ENCRYPTION_KEY on the server to store client secrets")
	}

	return encrypted, nil
}

func validateName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "name is required")
	}

	if len([]rune(name)) > maxNameLength {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "name is too long")
	}

	return name, nil
}

// validateIssuer accepts an absolute http or https URL. It is kept as entered, as
// the provider's configuration must name the very same issuer, trailing slash and all.
func validateIssuer(value string) (string, error) {
	issuer := strings.TrimSpace(value)

	parsed, err := url.Parse(issuer)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "issuer url must be an http or https URL")
	}

	return issuer, nil
}

func validateClientID(value string) (string, error) {
	clientID := strings.TrimSpace(value)
	if clientID == "" {
		return "", errors.InvalidInput(errors.ErrInvalidInput, "client id is required")
	}

	return clientID, nil
}

// defaultRole checks the role of accounts a provider creates; empty creates none.
func defaultRole(db *gorm.DB, id string) (*string, error) {
	if id == "" {
		return nil, nil
	}

	if _, err := common.FindByID[models.Role](db, id, errors.ErrRoleNotFound, "role not found"); err != nil {
		return nil, err
	}

	return &id, nil
}
