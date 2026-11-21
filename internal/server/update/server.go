package update

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/updates"
)

// Server implements the UpdateService
type Server struct {
	// checker is nil when the server doesn't check for releases.
	checker *updates.Checker
}

// NewServer creates a new UpdateService server
func NewServer(checker *updates.Checker) parlancev1connect.UpdateServiceHandler {
	return &Server{checker: checker}
}
