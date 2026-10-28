package activity

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
)

// Server implements the ActivityService
type Server struct {
	db *database.DB
}

// NewServer creates a new ActivityService server
func NewServer(db *database.DB) parlancev1connect.ActivityServiceHandler {
	return &Server{db: db}
}
