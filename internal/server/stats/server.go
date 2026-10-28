package stats

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
)

// Server implements the StatsService
type Server struct {
	db *database.DB
}

// NewServer creates a new StatsService server
func NewServer(db *database.DB) parlancev1connect.StatsServiceHandler {
	return &Server{db: db}
}
