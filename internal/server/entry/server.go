package entry

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
)

// Server implements the EntryService
type Server struct {
	db *database.DB
}

// NewServer creates a new EntryService server
func NewServer(db *database.DB) parlancev1connect.EntryServiceHandler {
	return &Server{db: db}
}
