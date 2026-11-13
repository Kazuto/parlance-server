package models

import (
	"log"

	"github.com/kazuto/parlance-server/internal/models"
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

	// Named by their codes, in any language
	locales := []models.Locale{
		{Code: "en", IsDefault: true},
		{Code: "de"},
		{Code: "fr"},
		{Code: "es"},
		{Code: "it"},
		{Code: "pt"},
		{Code: "ja"},
		{Code: "zh"},
	}

	for i := range locales {
		db.Where("code = ?", locales[i].Code).FirstOrCreate(&locales[i])
	}

	return nil
}

func (i *LocaleInitializer) Dependencies() []string { return []string{"Locales"} }
