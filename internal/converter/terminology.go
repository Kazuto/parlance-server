package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

func TerminologyToProto(t *models.Terminology) *pb.Terminology {
	if t == nil {
		return nil
	}

	return &pb.Terminology{
		Id:          t.ID,
		Term:        t.Term,
		Description: t.Description,
		Definitions: DefinitionsToProto(t.Definitions),
		CreatedAt:   t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   t.UpdatedAt.Format(time.RFC3339),
		DeletedAt:   formatDeletedAt(t.DeletedAt),
		CreatedBy:   derefString(t.CreatedBy),
		UpdatedBy:   derefString(t.UpdatedBy),
	}
}

func TerminologiesToProto(terminologies []models.Terminology) []*pb.Terminology {
	result := make([]*pb.Terminology, len(terminologies))
	for i, term := range terminologies {
		result[i] = TerminologyToProto(&term)
	}

	return result
}
