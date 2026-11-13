package converter

import (
	"testing"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
)

func TestLocaleToProtoNamesTheLanguageByItsCode(t *testing.T) {
	if LocaleToProto(nil, "en") != nil {
		t.Error("LocaleToProto(nil) should be nil")
	}

	now := time.Now()

	for _, tt := range []struct {
		code, requestLocale, want string
	}{
		{"de", "en", "German"},
		{"de", "de", "Deutsch"},
		{"en", "es", "Inglés"},
		{"pt-BR", "en", "Brazilian Portuguese"},
		{"pt_BR", "de", "Brasilianisches Portugiesisch"},
		// A language the request names nothing in, or none at all, gives English.
		{"de", "", "German"},
		{"de", "zz", "German"},
		// A made-up code names no language, so it stands for itself.
		{"en-informal", "en", "en-informal"},
	} {
		locale := &models.Locale{ID: "locale-id", Code: tt.code, IsDefault: true, CreatedAt: now, UpdatedAt: now}

		result := LocaleToProto(locale, tt.requestLocale)
		if result.Name != tt.want || result.Code != tt.code || !result.IsDefault {
			t.Errorf("LocaleToProto(%q, %q) = %v, want the name %q", tt.code, tt.requestLocale, result, tt.want)
		}
	}
}

func TestLocalesToProto(t *testing.T) {
	result := LocalesToProto([]models.Locale{{ID: "locale-1", Code: "en"}, {ID: "locale-2", Code: "de"}}, "en")

	if len(result) != 2 || result[0].Name != "English" || result[1].Name != "German" {
		t.Fatalf("LocalesToProto() = %v, want English and German", result)
	}
}
