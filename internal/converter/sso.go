package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// SSOProviderToProto converts an SSOProvider. The client secret stays out: only
// whether one is set is sent.
func SSOProviderToProto(provider *models.SSOProvider) *pb.SsoProvider {
	return &pb.SsoProvider{
		Id:              provider.ID,
		Name:            provider.Name,
		IssuerUrl:       provider.IssuerURL,
		ClientId:        provider.ClientID,
		HasClientSecret: provider.ClientSecret != "",
		DefaultRoleId:   provider.DefaultRoleID,
		Enabled:         provider.Enabled,
		CreatedAt:       provider.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       provider.UpdatedAt.Format(time.RFC3339),
	}
}
