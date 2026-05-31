package models

import (
	"log"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

type DefinitionSeeder struct{}

func NewDefinitionSeeder() *DefinitionSeeder { return &DefinitionSeeder{} }

func (i *DefinitionSeeder) Name() string { return "scopes" }
func (i *DefinitionSeeder) Description() string {
	return "Seed scopes"
}

func (i *DefinitionSeeder) Run(db *gorm.DB) error {
	log.Println("Seeding definitions...")

	var translator models.User
	db.Where("email = ?", "translator@parlance.dev").First(&translator)

	var terminologies []models.Terminology
	db.Find(&terminologies)

	var enLocale, deLocale, esLocale, frLocale models.Locale
	db.Where("code = ?", "en").First(&enLocale)
	db.Where("code = ?", "de").First(&deLocale)
	db.Where("code = ?", "es").First(&esLocale)
	db.Where("code = ?", "fr").First(&frLocale)

	// Definitions are the actual words to use in each locale for consistent terminology
	definitions := map[string]map[string]string{
		"Logo": {
			"en": "Logo",
			"de": "Logo", // NOT "Motif" - use Logo consistently
			"es": "Logo",
			"fr": "Logo",
		},
		"Dashboard": {
			"en": "Dashboard",
			"de": "Dashboard", // Keep English term, widely understood
			"es": "Panel de control",
			"fr": "Tableau de bord",
		},
		"User": {
			"en": "User",
			"de": "Benutzer",
			"es": "Usuario",
			"fr": "Utilisateur",
		},
		"Email": {
			"en": "Email",
			"de": "E-Mail",
			"es": "Correo electrónico",
			"fr": "Email",
		},
		"Password": {
			"en": "Password",
			"de": "Passwort",
			"es": "Contraseña",
			"fr": "Mot de passe",
		},
		"Settings": {
			"en": "Settings",
			"de": "Einstellungen",
			"es": "Configuración",
			"fr": "Paramètres",
		},
		"Profile": {
			"en": "Profile",
			"de": "Profil",
			"es": "Perfil",
			"fr": "Profil",
		},
		"API": {
			"en": "API",
			"de": "API", // Keep untranslated
			"es": "API",
			"fr": "API",
		},
		"Login": {
			"en": "Login",
			"de": "Anmelden", // German prefers verb form
			"es": "Iniciar sesión",
			"fr": "Connexion",
		},
		"Logout": {
			"en": "Logout",
			"de": "Abmelden",
			"es": "Cerrar sesión",
			"fr": "Déconnexion",
		},
	}

	localeMap := map[string]string{
		"en": enLocale.ID,
		"de": deLocale.ID,
		"es": esLocale.ID,
		"fr": frLocale.ID,
	}

	for _, terminology := range terminologies {
		if defs, exists := definitions[terminology.Term]; exists {
			for localeCode, translation := range defs {
				localeID := localeMap[localeCode]

				definition := models.Definition{
					TerminologyID: terminology.ID,
					LocaleID:      localeID,
					Translation:   translation,
					CreatedBy:     &translator.ID,
				}

				if err := db.Create(&definition).Error; err != nil {
					return err
				}
			}
			log.Printf("    Created definitions for: %s", terminology.Term)
		}
	}

	return nil
}

func (i *DefinitionSeeder) Dependencies() []string {
	return []string{"users", "locales", "terminologies"}
}
