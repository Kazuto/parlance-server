package user

import (
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/avatar"
	"github.com/kazuto/parlance-server/internal/database"
)

type Server struct {
	db      *database.DB
	avatars *avatar.Store
}

func NewServer(db *database.DB, avatars *avatar.Store) parlancev1connect.UserServiceHandler {
	return &Server{db: db, avatars: avatars}
}
