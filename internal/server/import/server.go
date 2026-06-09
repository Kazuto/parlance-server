package importservice

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
)

type Server struct {
	db *database.DB
	parlancev1connect.UnimplementedImportServiceHandler
}

func NewServer(db *database.DB) *Server {
	return &Server{db: db}
}
