// Package release serves releases, which hold entries back from exports until they go
// out, and publishes them.
package release

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
)

// Server implements the ReleaseService
type Server struct {
	db *database.DB
}

// NewServer creates a new ReleaseService server
func NewServer(db *database.DB) parlancev1connect.ReleaseServiceHandler {
	return &Server{db: db}
}
