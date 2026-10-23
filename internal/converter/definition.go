package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

func DefinitionToProto(d *models.Definition) *pb.Definition {
	if d == nil {
		return nil
	}

	return &pb.Definition{
		Id:            d.ID,
		TerminologyId: d.TerminologyID,
		LocaleId:      d.LocaleID,
		Translation:   d.Translation,
		CreatedAt:     d.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     d.UpdatedAt.Format(time.RFC3339),
		DeletedAt:     formatDeletedAt(d.DeletedAt),
		CreatedBy:     derefString(d.CreatedBy),
		UpdatedBy:     derefString(d.UpdatedBy),
	}
}

func DefinitionsToProto(definitions []models.Definition) []*pb.Definition {
	result := make([]*pb.Definition, len(definitions))
	for i, def := range definitions {
		result[i] = DefinitionToProto(&def)
	}

	return result
}

func DefinitionHistoryToProto(h *models.DefinitionHistory) *pb.DefinitionHistory {
	if h == nil {
		return nil
	}

	return &pb.DefinitionHistory{
		Id:            h.ID,
		DefinitionId:  h.DefinitionID,
		LocaleId:      h.LocaleID,
		TerminologyId: h.TerminologyID,
		UserId:        derefString(h.UserID),
		Translation:   h.Translation,
		Action:        h.Action,
		ChangedAt:     h.ChangedAt.Format(time.RFC3339),
		User:          UserToProto(h.User),
	}
}

func DefinitionHistoriesToProto(histories []models.DefinitionHistory) []*pb.DefinitionHistory {
	result := make([]*pb.DefinitionHistory, len(histories))
	for i, history := range histories {
		result[i] = DefinitionHistoryToProto(&history)
	}

	return result
}
