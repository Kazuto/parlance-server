package scope

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
)

// Server implements the ScopeService
type Server struct {
	db *database.DB
}

// NewServer creates a new ScopeService server
func NewServer(db *database.DB) parlancev1connect.ScopeServiceHandler {
	return &Server{db: db}
}
