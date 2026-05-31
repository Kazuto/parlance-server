package models

import (
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

type LocalizationsSeeder struct{}

func NewLocalizationsSeeder() *LocalizationsSeeder { return &LocalizationsSeeder{} }

func (i *LocalizationsSeeder) Name() string { return "Localizations" }

func (i *LocalizationsSeeder) Run(db *gorm.DB) error {
	var enLocale, deLocale, esLocale, frLocale models.Locale
	db.Where("code = ?", "en").First(&enLocale)
	db.Where("code = ?", "de").First(&deLocale)
	db.Where("code = ?", "es").First(&esLocale)
	db.Where("code = ?", "fr").First(&frLocale)

	var translator models.User
	db.Where("email = ?", "translator@parlance.dev").First(&translator)

	var entries []models.Entry
	db.Limit(20).Find(&entries)

	translations := map[string]map[string]string{
		"app.name": {
			"en": "Parlance",
			"de": "Parlance",
			"es": "Parlance",
			"fr": "Parlance",
		},
		"app.tagline": {
			"en": "Translation Management Made Simple",
			"de": "Übersetzungsverwaltung leicht gemacht",
			"es": "Gestión de traducciones simplificada",
			"fr": "Gestion des traductions simplifiée",
		},
		"button.submit": {
			"en": "Submit",
			"de": "Absenden",
			"es": "Enviar",
			"fr": "Soumettre",
		},
		"button.cancel": {
			"en": "Cancel",
			"de": "Abbrechen",
			"es": "Cancelar",
			"fr": "Annuler",
		},
		"button.save": {
			"en": "Save",
			"de": "Speichern",
			"es": "Guardar",
			"fr": "Enregistrer",
		},
		"button.delete": {
			"en": "Delete",
			"de": "Löschen",
			"es": "Eliminar",
			"fr": "Supprimer",
		},
		"nav.home": {
			"en": "Home",
			"de": "Startseite",
			"es": "Inicio",
			"fr": "Accueil",
		},
		"nav.settings": {
			"en": "Settings",
			"de": "Einstellungen",
			"es": "Configuración",
			"fr": "Paramètres",
		},
		"nav.profile": {
			"en": "Profile",
			"de": "Profil",
			"es": "Perfil",
			"fr": "Profil",
		},
		"error.not_found": {
			"en": "Page not found",
			"de": "Seite nicht gefunden",
			"es": "Página no encontrada",
			"fr": "Page non trouvée",
		},
		"error.unauthorized": {
			"en": "Unauthorized access",
			"de": "Unbefugter Zugriff",
			"es": "Acceso no autorizado",
			"fr": "Accès non autorisé",
		},
		"error.server_error": {
			"en": "Internal server error",
			"de": "Interner Serverfehler",
			"es": "Error interno del servidor",
			"fr": "Erreur interne du serveur",
		},
		"login.title": {
			"en": "Sign In",
			"de": "Anmelden",
			"es": "Iniciar sesión",
			"fr": "Se connecter",
		},
		"login.email": {
			"en": "Email",
			"de": "E-Mail",
			"es": "Correo electrónico",
			"fr": "Email",
		},
		"login.password": {
			"en": "Password",
			"de": "Passwort",
			"es": "Contraseña",
			"fr": "Mot de passe",
		},
		"login.forgot_password": {
			"en": "Forgot password?",
			"de": "Passwort vergessen?",
			"es": "¿Olvidaste tu contraseña?",
			"fr": "Mot de passe oublié?",
		},
		"validation.required": {
			"en": "This field is required",
			"de": "Dieses Feld ist erforderlich",
			"es": "Este campo es obligatorio",
			"fr": "Ce champ est requis",
		},
		"validation.email": {
			"en": "Please enter a valid email address",
			"de": "Bitte geben Sie eine gültige E-Mail-Adresse ein",
			"es": "Por favor, introduce una dirección de correo electrónico válida",
			"fr": "Veuillez saisir une adresse email valide",
		},
		"notification.success": {
			"en": "Operation completed successfully",
			"de": "Vorgang erfolgreich abgeschlossen",
			"es": "Operación completada con éxito",
			"fr": "Opération terminée avec succès",
		},
		"notification.error": {
			"en": "An error occurred",
			"de": "Ein Fehler ist aufgetreten",
			"es": "Se produjo un error",
			"fr": "Une erreur s'est produite",
		},
	}

	localeMap := map[string]string{
		"en": enLocale.ID,
		"de": deLocale.ID,
		"es": esLocale.ID,
		"fr": frLocale.ID,
	}

	for _, entry := range entries {
		if trans, exists := translations[entry.Key]; exists {
			for localeCode, translation := range trans {
				localeID := localeMap[localeCode]

				localization := models.Localization{
					EntryID:     entry.ID,
					LocaleID:    localeID,
					Translation: translation,
					CreatedBy:   &translator.ID,
				}

				db.Where("entry_id = ? AND locale_id = ?", entry.ID, localeID).FirstOrCreate(&localization)
			}
		}
	}

	return nil
}

func (i *LocalizationsSeeder) Dependencies() []string { return []string{"users", "locales", "entries"} }
