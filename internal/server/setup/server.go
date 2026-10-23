package setup

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
)

type Server struct {
	db     *database.DB
	config *config.Config
}

func NewServer(db *database.DB, config *config.Config) parlancev1connect.SetupServiceHandler {
	return &Server{
		db:     db,
		config: config,
	}
}
