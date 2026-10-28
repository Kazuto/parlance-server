package role

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
)

// Server implements the RoleService
type Server struct {
	db *database.DB
}

// NewServer creates a new RoleService server
func NewServer(db *database.DB) parlancev1connect.RoleServiceHandler {
	return &Server{db: db}
}
