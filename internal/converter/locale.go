package converter

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/translate"
)

// formalities maps a locale's form of address to its proto enum; empty is unspecified
var formalities = map[string]pb.Formality{
	string(translate.Formal):   pb.Formality_FORMALITY_FORMAL,
	string(translate.Informal): pb.Formality_FORMALITY_INFORMAL,
}

// FormalityFromProto returns the locale's form of address, "" for either, and
// false for a value it doesn't know
func FormalityFromProto(value pb.Formality) (string, bool) {
	if value == pb.Formality_FORMALITY_UNSPECIFIED {
		return "", true
	}

	formality := lookup(formalities, value)

	return formality, formality != ""
}

// LocaleToProto converts a database Locale model to protobuf
// requestLocale is the locale code for localizing the name field (e.g., "de", "en")
func LocaleToProto(l *models.Locale, requestLocale string) *pb.Locale {
	if l == nil {
		return nil
	}

	// Parse names from JSONB (datatypes.JSON is []byte)
	var names map[string]string
	if l.Names != nil {
		if err := json.Unmarshal(l.Names, &names); err != nil {
			names = make(map[string]string)
		}
	} else {
		names = make(map[string]string)
	}

	// Get localized name based on request locale, fallback to English, then first available
	localizedName := names["en"] // Default to English

	if requestLocale != "" {
		if name, ok := names[requestLocale]; ok {
			localizedName = name
		}
	}
	// If still empty, use first available name
	if localizedName == "" {
		for _, name := range names {
			localizedName = name

			break
		}
	}

	return &pb.Locale{
		Id:        l.ID,
		Code:      l.Code,
		Name:      localizedName,
		Names:     names,
		IsDefault: l.IsDefault,
		Formality: formalities[l.Formality],
		CreatedAt: l.CreatedAt.Format(time.RFC3339),
		UpdatedAt: l.UpdatedAt.Format(time.RFC3339),
		DeletedAt: formatDeletedAt(l.DeletedAt),
	}
}

// LocalesToProto converts a slice of Locale models to protobuf
func LocalesToProto(locales []models.Locale, requestLocale string) []*pb.Locale {
	result := make([]*pb.Locale, len(locales))
	for i, locale := range locales {
		result[i] = LocaleToProto(&locale, requestLocale)
	}

	return result
}

// NamesMapToJSON converts a proto names map to JSONB
func NamesMapToJSON(names map[string]string) (datatypes.JSON, error) {
	if names == nil {
		names = make(map[string]string)
	}

	data, err := json.Marshal(names)
	if err != nil {
		return nil, err
	}

	return datatypes.JSON(data), nil
}
