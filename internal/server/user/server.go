package user

import (
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
)

// Server implements the UserService
type Server struct {
	db *database.DB
}

// NewServer creates a new UserService server
func NewServer(db *database.DB) parlancev1connect.UserServiceHandler {
	return &Server{db: db}
}
