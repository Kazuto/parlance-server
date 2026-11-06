package localization

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/translators"
)

// Server implements the LocalizationService
type Server struct {
	db          *database.DB
	config      *config.Config
	translators translators.Source
}

// NewServer creates a new LocalizationService server
func NewServer(db *database.DB, cfg *config.Config, source translators.Source) parlancev1connect.LocalizationServiceHandler {
	return &Server{
		db:          db,
		config:      cfg,
		translators: source,
	}
}
