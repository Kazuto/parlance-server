package auth

import (
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
)

// Server implements the AuthService
type Server struct {
	db     *database.DB
	config *config.Config
}

// NewServer creates a new AuthService server
func NewServer(db *database.DB, cfg *config.Config) parlancev1connect.AuthServiceHandler {
	return &Server{
		db:     db,
		config: cfg,
	}
}
