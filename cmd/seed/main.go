package main

import (
	"encoding/json"
	"log"

	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/config"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/datatypes"
)

func main() {
	log.Println("🌱 Starting database seeding...")

	// Load config
	cfg := config.Load()

	// Connect to database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations
	log.Println("Running database migrations...")
	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Run seeds
	if err := seedRolesAndPermissions(db); err != nil {
		log.Fatalf("Failed to seed roles and permissions: %v", err)
	}

	if err := seedUsers(db); err != nil {
		log.Fatalf("Failed to seed users: %v", err)
	}

	if err := seedLocales(db); err != nil {
		log.Fatalf("Failed to seed locales: %v", err)
	}

	if err := seedScopes(db); err != nil {
		log.Fatalf("Failed to seed scopes: %v", err)
	}

	if err := seedEntries(db); err != nil {
		log.Fatalf("Failed to seed entries: %v", err)
	}

	if err := seedLocalizations(db); err != nil {
		log.Fatalf("Failed to seed localizations: %v", err)
	}

	if err := seedTerminologies(db); err != nil {
		log.Fatalf("Failed to seed terminologies: %v", err)
	}

	if err := seedDefinitions(db); err != nil {
		log.Fatalf("Failed to seed definitions: %v", err)
	}

	log.Println("✅ Database seeding completed successfully!")
}

func seedRolesAndPermissions(db *database.DB) error {
	log.Println("  Seeding roles and permissions...")

	// Create permissions
	permissions := []models.Permission{
		{Name: "create_entry", Resource: "entry", Action: "create"},
		{Name: "read_entry", Resource: "entry", Action: "read"},
		{Name: "update_entry", Resource: "entry", Action: "update"},
		{Name: "delete_entry", Resource: "entry", Action: "delete"},
		{Name: "create_locale", Resource: "locale", Action: "create"},
		{Name: "manage_users", Resource: "user", Action: "manage"},
		{Name: "manage_terminology", Resource: "terminology", Action: "manage"},
	}

	for i := range permissions {
		if err := db.FirstOrCreate(&permissions[i], models.Permission{Name: permissions[i].Name}).Error; err != nil {
			return err
		}
	}

	// Create roles
	adminRole := models.Role{Name: "admin", Permissions: permissions}
	if err := db.FirstOrCreate(&adminRole, models.Role{Name: "admin"}).Error; err != nil {
		return err
	}
	db.Model(&adminRole).Association("Permissions").Replace(&permissions)

	translatorRole := models.Role{
		Name: "translator",
		Permissions: []models.Permission{
			permissions[0], permissions[1], permissions[2], // create, read, update entry
		},
	}
	if err := db.FirstOrCreate(&translatorRole, models.Role{Name: "translator"}).Error; err != nil {
		return err
	}
	db.Model(&translatorRole).Association("Permissions").Replace(translatorRole.Permissions)

	viewerRole := models.Role{Name: "viewer", Permissions: []models.Permission{permissions[1]}}
	if err := db.FirstOrCreate(&viewerRole, models.Role{Name: "viewer"}).Error; err != nil {
		return err
	}
	db.Model(&viewerRole).Association("Permissions").Replace(viewerRole.Permissions)

	return nil
}

func seedUsers(db *database.DB) error {
	log.Println("  Seeding users...")

	// Get roles
	var adminRole, translatorRole, viewerRole models.Role
	db.Where("name = ?", "admin").First(&adminRole)
	db.Where("name = ?", "translator").First(&translatorRole)
	db.Where("name = ?", "viewer").First(&viewerRole)

	users := []struct {
		email    string
		password string
		name     string
		roles    []models.Role
	}{
		{"admin@parlance.dev", "admin123", "Admin User", []models.Role{adminRole}},
		{"translator@parlance.dev", "translator123", "Translator User", []models.Role{translatorRole}},
		{"viewer@parlance.dev", "viewer123", "Viewer User", []models.Role{viewerRole}},
		{"john@example.com", "password123", "John Doe", []models.Role{translatorRole}},
		{"jane@example.com", "password123", "Jane Smith", []models.Role{translatorRole}},
	}

	for _, userData := range users {
		hashedPassword, _ := auth.HashPassword(userData.password)
		user := models.User{
			Email:        userData.email,
			PasswordHash: hashedPassword,
			Name:         userData.name,
		}

		if err := db.Create(&user).Error; err != nil {
			return err
		}

		if err := db.Model(&user).Association("Roles").Replace(&userData.roles); err != nil {
			return err
		}

		log.Printf("    Created user: %s (password: %s)", userData.email, userData.password)
	}

	return nil
}

func seedLocales(db *database.DB) error {
	log.Println("  Seeding locales...")

	locales := []struct {
		code      string
		names     map[string]string
		isDefault bool
	}{
		{
			code: "en",
			names: map[string]string{
				"en": "English",
				"de": "Englisch",
				"es": "Inglés",
				"fr": "Anglais",
			},
			isDefault: true,
		},
		{
			code: "de",
			names: map[string]string{
				"en": "German",
				"de": "Deutsch",
				"es": "Alemán",
				"fr": "Allemand",
			},
			isDefault: false,
		},
		{
			code: "es",
			names: map[string]string{
				"en": "Spanish",
				"de": "Spanisch",
				"es": "Español",
				"fr": "Espagnol",
			},
			isDefault: false,
		},
		{
			code: "fr",
			names: map[string]string{
				"en": "French",
				"de": "Französisch",
				"es": "Francés",
				"fr": "Français",
			},
			isDefault: false,
		},
		{
			code: "ja",
			names: map[string]string{
				"en": "Japanese",
				"de": "Japanisch",
				"es": "Japonés",
				"fr": "Japonais",
			},
			isDefault: false,
		},
	}

	for _, localeData := range locales {
		namesJSON, _ := json.Marshal(localeData.names)
		locale := models.Locale{
			Code:      localeData.code,
			Names:     datatypes.JSON(namesJSON),
			IsDefault: localeData.isDefault,
		}

		if err := db.Create(&locale).Error; err != nil {
			return err
		}

		log.Printf("    Created locale: %s", localeData.code)
	}

	return nil
}

func seedScopes(db *database.DB) error {
	log.Println("  Seeding scopes...")

	scopes := []struct {
		name        string
		description string
	}{
		{"frontend", "Frontend web application translations"},
		{"backend", "Backend API messages and errors"},
		{"mobile", "Mobile app translations"},
		{"email", "Email templates and notifications"},
		{"marketing", "Marketing content and landing pages"},
		{"docs", "Documentation and help content"},
	}

	for _, scopeData := range scopes {
		scope := models.Scope{
			Name:        scopeData.name,
			Slug:        scopeData.name,
			Description: scopeData.description,
		}

		if err := db.Create(&scope).Error; err != nil {
			return err
		}

		log.Printf("    Created scope: %s", scopeData.name)
	}

	return nil
}

func seedEntries(db *database.DB) error {
	log.Println("  Seeding entries...")

	// Get admin user for CreatedBy
	var adminUser models.User
	db.Where("email = ?", "admin@parlance.dev").First(&adminUser)

	// Get scopes
	var frontendScope, backendScope, mobileScope models.Scope
	db.Where("name = ?", "frontend").First(&frontendScope)
	db.Where("name = ?", "backend").First(&backendScope)
	db.Where("name = ?", "mobile").First(&mobileScope)

	entries := []struct {
		key         string
		description string
		scopes      []models.Scope
	}{
		{"app.name", "Application name", []models.Scope{frontendScope, mobileScope}},
		{"app.tagline", "Application tagline", []models.Scope{frontendScope}},
		{"button.submit", "Submit button label", []models.Scope{frontendScope, mobileScope}},
		{"button.cancel", "Cancel button label", []models.Scope{frontendScope, mobileScope}},
		{"button.save", "Save button label", []models.Scope{frontendScope, mobileScope}},
		{"button.delete", "Delete button label", []models.Scope{frontendScope, mobileScope}},
		{"nav.home", "Navigation: Home", []models.Scope{frontendScope, mobileScope}},
		{"nav.settings", "Navigation: Settings", []models.Scope{frontendScope, mobileScope}},
		{"nav.profile", "Navigation: Profile", []models.Scope{frontendScope, mobileScope}},
		{"error.not_found", "404 error message", []models.Scope{frontendScope, backendScope}},
		{"error.unauthorized", "401 error message", []models.Scope{frontendScope, backendScope}},
		{"error.server_error", "500 error message", []models.Scope{frontendScope, backendScope}},
		{"login.title", "Login page title", []models.Scope{frontendScope, mobileScope}},
		{"login.email", "Email field label", []models.Scope{frontendScope, mobileScope}},
		{"login.password", "Password field label", []models.Scope{frontendScope, mobileScope}},
		{"login.forgot_password", "Forgot password link", []models.Scope{frontendScope, mobileScope}},
		{"validation.required", "Field required validation message", []models.Scope{frontendScope, backendScope, mobileScope}},
		{"validation.email", "Invalid email validation message", []models.Scope{frontendScope, backendScope, mobileScope}},
		{"notification.success", "Generic success notification", []models.Scope{frontendScope, mobileScope}},
		{"notification.error", "Generic error notification", []models.Scope{frontendScope, mobileScope}},
	}

	for _, entryData := range entries {
		entry := models.Entry{
			Key:         entryData.key,
			Description: entryData.description,
			CreatedBy:   &adminUser.ID,
		}

		if err := db.Create(&entry).Error; err != nil {
			return err
		}

		if len(entryData.scopes) > 0 {
			db.Model(&entry).Association("Scopes").Replace(&entryData.scopes)
		}

		log.Printf("    Created entry: %s", entryData.key)
	}

	return nil
}

func seedLocalizations(db *database.DB) error {
	log.Println("  Seeding localizations...")

	// Get locales
	var enLocale, deLocale, esLocale, frLocale models.Locale
	db.Where("code = ?", "en").First(&enLocale)
	db.Where("code = ?", "de").First(&deLocale)
	db.Where("code = ?", "es").First(&esLocale)
	db.Where("code = ?", "fr").First(&frLocale)

	// Get translator user
	var translator models.User
	db.Where("email = ?", "translator@parlance.dev").First(&translator)

	// Get entries
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

				if err := db.Create(&localization).Error; err != nil {
					return err
				}
			}
			log.Printf("    Created localizations for: %s", entry.Key)
		}
	}

	return nil
}

func seedTerminologies(db *database.DB) error {
	log.Println("  Seeding terminologies...")

	var adminUser models.User
	db.Where("email = ?", "admin@parlance.dev").First(&adminUser)

	terminologies := []struct {
		term        string
		description string
	}{
		{"Logo", "Company or product logo/branding"},
		{"Dashboard", "Main application dashboard view"},
		{"User", "Application user or account holder"},
		{"Email", "Email address or email communication"},
		{"Password", "Authentication password"},
		{"Settings", "Application settings and configuration"},
		{"Profile", "User profile information"},
		{"API", "Application Programming Interface"},
		{"Login", "Authentication/sign-in action"},
		{"Logout", "Sign-out action"},
	}

	for _, termData := range terminologies {
		terminology := models.Terminology{
			Term:        termData.term,
			Description: termData.description,
			CreatedBy:   &adminUser.ID,
		}

		if err := db.Create(&terminology).Error; err != nil {
			return err
		}

		log.Printf("    Created terminology: %s", termData.term)
	}

	return nil
}

func seedDefinitions(db *database.DB) error {
	log.Println("  Seeding definitions...")

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
