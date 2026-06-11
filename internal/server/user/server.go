package user

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
)

type Server struct {
	db       *database.DB
	s3       *s3.Client
	bucket   string
	endpoint string
}

func NewServer(db *database.DB, s3 *s3.Client, cfg config.S3Config) parlancev1connect.UserServiceHandler {
	return &Server{db: db, s3: s3, bucket: cfg.Bucket, endpoint: cfg.Endpoint}
}
