package converter

import (
	"cmp"
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/langnames"
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

// LocaleToProto converts a database Locale model to protobuf. requestLocale is the
// language its name is given in, e.g. "de"; the name falls back to English, then to
// the code.
func LocaleToProto(l *models.Locale, requestLocale string) *pb.Locale {
	if l == nil {
		return nil
	}

	return &pb.Locale{
		Id:        l.ID,
		Code:      l.Code,
		Name:      cmp.Or(langnames.Name(l.Code, requestLocale), langnames.Name(l.Code, "en"), l.Code),
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
