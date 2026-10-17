package setup

import (
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
	pb "github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
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
