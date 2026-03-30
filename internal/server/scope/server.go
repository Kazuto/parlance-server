package scope

import (
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
)

// Server implements the ScopeService
type Server struct {
	db *database.DB
}

// NewServer creates a new ScopeService server
func NewServer(db *database.DB) parlancev1connect.ScopeServiceHandler {
	return &Server{db: db}
}
