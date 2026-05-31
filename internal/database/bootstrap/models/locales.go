package models

import (
	"log"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type LocaleInitializer struct{}

func NewLocaleInitializer() *LocaleInitializer { return &LocaleInitializer{} }

func (i *LocaleInitializer) Name() string { return "Locales" }
func (i *LocaleInitializer) Description() string {
	return "Populates Locales table"
}

func (i *LocaleInitializer) Run(db *gorm.DB) error {
	log.Println("Initializing locales...")

	locales := []models.Locale{
		{Code: "en", Names: []byte(`{"en":"English","de":"Englisch","fr":"Anglais","es":"Inglés","it":"Inglese","pt":"Inglês","ja":"英語","zh":"英语"}`), IsDefault: true},
		{Code: "de", Names: []byte(`{"en":"German","de":"Deutsch","fr":"Allemand","es":"Alemán","it":"Tedesco","pt":"Alemão","ja":"ドイツ語","zh":"德语"}`), IsDefault: false},
		{Code: "fr", Names: []byte(`{"en":"French","de":"Französisch","fr":"Français","es":"Francés","it":"Francese","pt":"Francês","ja":"フランス語","zh":"法语"}`), IsDefault: false},
		{Code: "es", Names: []byte(`{"en":"Spanish","de":"Spanisch","fr":"Espagnol","es":"Español","it":"Spagnolo","pt":"Espanhol","ja":"スペイン語","zh":"西班牙语"}`), IsDefault: false},
		{Code: "it", Names: []byte(`{"en":"Italian","de":"Italienisch","fr":"Italien","es":"Italiano","it":"Italiano","pt":"Italiano","ja":"イタリア語","zh":"意大利语"}`), IsDefault: false},
		{Code: "pt", Names: []byte(`{"en":"Portuguese","de":"Portugiesisch","fr":"Portugais","es":"Portugués","it":"Portoghese","pt":"Português","ja":"ポルトガル語","zh":"葡萄牙语"}`), IsDefault: false},
		{Code: "ja", Names: []byte(`{"en":"Japanese","de":"Japanisch","fr":"Japonais","es":"Japonés","it":"Giapponese","pt":"Japonês","ja":"日本語","zh":"日语"}`), IsDefault: false},
		{Code: "zh", Names: []byte(`{"en":"Chinese","de":"Chinesisch","fr":"Chinois","es":"Chino","it":"Cinese","pt":"Chinês","ja":"中国語","zh":"中文"}`), IsDefault: false},
	}

	for i := range locales {
		if err := db.FirstOrCreate(&locales[i], models.Locale{
			Code:      locales[i].Code,
			Names:     datatypes.JSON(locales[i].Names),
			IsDefault: locales[i].IsDefault,
		}).Error; err != nil {
			return err
		}
	}

	return nil
}

func (i *LocaleInitializer) Dependencies() []string { return []string{"Locales"} }
