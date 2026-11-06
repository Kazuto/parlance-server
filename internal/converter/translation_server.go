package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// TranslationServerToProto converts a TranslationServer. The API key stays out:
// only whether one is set is sent.
func TranslationServerToProto(s *models.TranslationServer) *pb.TranslationServer {
	if s == nil {
		return nil
	}

	return &pb.TranslationServer{
		Id:             s.ID,
		Name:           s.Name,
		BaseUrl:        s.BaseURL,
		Model:          s.Model,
		HasApiKey:      s.APIKey != "",
		MaxConcurrency: int32(s.MaxConcurrency),
		Enabled:        s.Enabled,
		CreatedAt:      s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      s.UpdatedAt.Format(time.RFC3339),
	}
}

// TranslationServersToProto converts a slice of TranslationServers
func TranslationServersToProto(servers []models.TranslationServer) []*pb.TranslationServer {
	result := make([]*pb.TranslationServer, len(servers))
	for i, server := range servers {
		result[i] = TranslationServerToProto(&server)
	}

	return result
}
