package terminology

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
)

type Server struct {
	db *database.DB
}

func NewServer(db *database.DB) parlancev1connect.TerminologyServiceHandler {
	return &Server{
		db: db,
	}
}
