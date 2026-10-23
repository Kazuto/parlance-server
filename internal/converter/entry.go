package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// EntryToProto converts a database Entry model to protobuf
func EntryToProto(e *models.Entry) *pb.Entry {
	if e == nil {
		return nil
	}

	return &pb.Entry{
		Id:            e.ID,
		Key:           e.Key,
		Description:   e.Description,
		CreatedAt:     e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     e.UpdatedAt.Format(time.RFC3339),
		DeletedAt:     formatDeletedAt(e.DeletedAt),
		CreatedBy:     derefString(e.CreatedBy),
		UpdatedBy:     derefString(e.UpdatedBy),
		Localizations: LocalizationsToProto(e.Localizations),
		Scopes:        ScopesToProto(e.Scopes),
	}
}

// EntriesToProto converts a slice of Entry models to protobuf
func EntriesToProto(entries []models.Entry) []*pb.Entry {
	result := make([]*pb.Entry, len(entries))
	for i, entry := range entries {
		result[i] = EntryToProto(&entry)
	}

	return result
}

// LocalizationToProto converts a database Localization model to protobuf
func LocalizationToProto(l *models.Localization) *pb.Localization {
	if l == nil {
		return nil
	}

	return &pb.Localization{
		Id:            l.ID,
		EntryId:       l.EntryID,
		LocaleId:      l.LocaleID,
		Translation:   l.Translation,
		CreatedAt:     l.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     l.UpdatedAt.Format(time.RFC3339),
		CreatedBy:     derefString(l.CreatedBy),
		UpdatedBy:     derefString(l.UpdatedBy),
		Status:        ReviewStatusToProto(l.Status),
		ReviewedBy:    derefString(l.ReviewedBy),
		ReviewedAt:    formatTime(l.ReviewedAt),
		ReviewComment: l.ReviewComment,
	}
}

// LocalizationsToProto converts a slice of Localization models to protobuf
func LocalizationsToProto(localizations []models.Localization) []*pb.Localization {
	result := make([]*pb.Localization, len(localizations))
	for i, loc := range localizations {
		result[i] = LocalizationToProto(&loc)
	}

	return result
}

// LocalizationHistoryToProto converts a database LocalizationHistory model to protobuf
func LocalizationHistoryToProto(h *models.LocalizationHistory) *pb.LocalizationHistory {
	if h == nil {
		return nil
	}

	return &pb.LocalizationHistory{
		Id:             h.ID,
		LocalizationId: h.LocalizationID,
		LocaleId:       h.LocaleID,
		EntryId:        h.EntryID,
		UserId:         derefString(h.UserID),
		Translation:    h.Translation,
		Action:         h.Action,
		ChangedAt:      h.ChangedAt.Format(time.RFC3339),
		User:           UserToProto(h.User),
		Status:         ReviewStatusToProto(h.Status),
		Comment:        h.Comment,
	}
}

// LocalizationHistoriesToProto converts a slice of LocalizationHistory models to protobuf
func LocalizationHistoriesToProto(histories []models.LocalizationHistory) []*pb.LocalizationHistory {
	result := make([]*pb.LocalizationHistory, len(histories))
	for i, history := range histories {
		result[i] = LocalizationHistoryToProto(&history)
	}

	return result
}
