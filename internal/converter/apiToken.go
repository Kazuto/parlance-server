package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// APITokenToProto converts an API token, which never includes the token itself
func APITokenToProto(token *models.APIToken) *pb.ApiToken {
	return &pb.ApiToken{
		Id:         token.ID,
		Name:       token.Name,
		Prefix:     token.Prefix,
		CreatedAt:  token.CreatedAt.UTC().Format(time.RFC3339),
		LastUsedAt: optionalTime(token.LastUsedAt),
		ExpiresAt:  optionalTime(token.ExpiresAt),
	}
}
