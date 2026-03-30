package locale

import (
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
)

// Server implements the LocaleService
type Server struct {
	db *database.DB
}

// NewServer creates a new LocaleService server
func NewServer(db *database.DB) parlancev1connect.LocaleServiceHandler {
	return &Server{db: db}
}
