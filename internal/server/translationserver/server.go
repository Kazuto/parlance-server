package translationserver

import (
	"net/url"

	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/secret"
	"github.com/kazuto/parlance-server/internal/translate"
)

// maxConcurrencyLimit caps how many requests one server may be given at once.
const maxConcurrencyLimit = 64

// Server implements the TranslationServerService
type Server struct {
	db  *database.DB
	box *secret.Box
	// newTranslator connects to a server for TestTranslationServer.
	newTranslator func(translate.Endpoint) translate.Translator
}

// NewServer creates a new TranslationServerService server
func NewServer(db *database.DB, box *secret.Box) parlancev1connect.TranslationServerServiceHandler {
	return &Server{
		db:            db,
		box:           box,
		newTranslator: translate.NewClientFor,
	}
}

// validateBaseURL accepts an absolute http or https URL.
func validateBaseURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.InvalidInput(errors.ErrInvalidInput, "base url must be an http or https URL")
	}

	return nil
}

func validateMaxConcurrency(value int32) error {
	if value < 1 || value > maxConcurrencyLimit {
		return errors.InvalidInput(errors.ErrInvalidInput, "max concurrency must be between 1 and 64")
	}

	return nil
}

// encryptAPIKey returns the key to store: encrypted, or empty for no key.
func (s *Server) encryptAPIKey(key string) (string, error) {
	if key == "" {
		return "", nil
	}

	encrypted, err := s.box.Encrypt(key)
	if err != nil {
		return "", errors.FailedPrecondition(errors.ErrEncryptionKeyMissing, "set ENCRYPTION_KEY on the server to store API keys")
	}

	return encrypted, nil
}
