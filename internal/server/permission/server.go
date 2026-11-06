package permission

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
)

// Server implements the PermissionService
type Server struct {
	db *database.DB
}

// NewServer creates a new PermissionService server
func NewServer(db *database.DB) parlancev1connect.PermissionServiceHandler {
	return &Server{db: db}
}
