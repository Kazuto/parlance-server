package converter

import (
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// EntryToProto converts a database Entry model to protobuf
func EntryToProto(e *models.Entry, requestLocale string) *pb.Entry {
	if e == nil {
		return nil
	}

	localizations := make([]*pb.Localization, len(e.Localizations))
	for i, loc := range e.Localizations {
		localizations[i] = LocalizationToProto(&loc)
	}

	scopes := make([]*pb.Scope, len(e.Scopes))
	for i, scope := range e.Scopes {
		scopes[i] = ScopeToProto(&scope)
	}

	createdBy := ""
	if e.CreatedBy != nil {
		createdBy = *e.CreatedBy
	}

	updatedBy := ""
	if e.UpdatedBy != nil {
		updatedBy = *e.UpdatedBy
	}

	deletedAt := ""
	if e.DeletedAt.Valid {
		deletedAt = e.DeletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}

	return &pb.Entry{
		Id:            e.ID,
		Key:           e.Key,
		Description:   e.Description,
		CreatedAt:     e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     e.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		DeletedAt:     deletedAt,
		CreatedBy:     createdBy,
		UpdatedBy:     updatedBy,
		Localizations: localizations,
		Scopes:        scopes,
	}
}

// EntriesToProto converts a slice of Entry models to protobuf
func EntriesToProto(entries []models.Entry, requestLocale string) []*pb.Entry {
	result := make([]*pb.Entry, len(entries))
	for i, entry := range entries {
		result[i] = EntryToProto(&entry, requestLocale)
	}

	return result
}

// LocalizationToProto converts a database Localization model to protobuf
func LocalizationToProto(l *models.Localization) *pb.Localization {
	if l == nil {
		return nil
	}

	createdBy := ""
	if l.CreatedBy != nil {
		createdBy = *l.CreatedBy
	}

	updatedBy := ""
	if l.UpdatedBy != nil {
		updatedBy = *l.UpdatedBy
	}

	return &pb.Localization{
		Id:          l.ID,
		EntryId:     l.EntryID,
		LocaleId:    l.LocaleID,
		Translation: l.Translation,
		CreatedAt:   l.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   l.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedBy:   createdBy,
		UpdatedBy:   updatedBy,
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

	userID := ""
	if h.UserID != nil {
		userID = *h.UserID
	}

	return &pb.LocalizationHistory{
		Id:             h.ID,
		LocalizationId: h.LocalizationID,
		LocaleId:       h.LocaleID,
		EntryId:        h.EntryID,
		UserId:         userID,
		Translation:    h.Translation,
		Action:         h.Action,
		ChangedAt:      h.ChangedAt.Format("2006-01-02T15:04:05Z07:00"),
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
