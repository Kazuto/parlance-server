package setup

import (
	pb "github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
)

type Server struct {
	pb.UnimplementedSetupServiceHandler
	db     *database.DB
	config *config.Config
}

func NewServer(db *database.DB, config *config.Config) *Server {
	return &Server{
		db:     db,
		config: config,
	}
}
