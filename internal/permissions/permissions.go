package permissions

// Permission name constants - single source of truth for all permission strings
const (
	// Locale permissions
	CreateLocale          = "create_locale"
	ReadLocale            = "read_locale"
	UpdateLocale          = "update_locale"
	DeleteLocale          = "delete_locale"
	PermanentDeleteLocale = "permanent_delete_locale"

	// Entry permissions
	CreateEntry = "create_entry"
	ReadEntry   = "read_entry"
	UpdateEntry = "update_entry"
	DeleteEntry = "delete_entry"
	// Approving or rejecting translations
	ReviewEntry = "review_entry"
	// Importing and exporting translation files
	ImportEntry = "import_entry"
	ExportEntry = "export_entry"

	// User permissions
	CreateUser = "create_user"
	ReadUser   = "read_user"
	UpdateUser = "update_user"
	DeleteUser = "delete_user"

	// Role permissions
	CreateRole = "create_role"
	ReadRole   = "read_role"
	UpdateRole = "update_role"
	DeleteRole = "delete_role"

	// Permission permissions
	ReadPermission = "read_permission"

	// Terminology permissions
	CreateTerminology = "create_terminology"
	ReadTerminology   = "read_terminology"
	UpdateTerminology = "update_terminology"
	DeleteTerminology = "delete_terminology"

	// Scope permissions
	CreateScope = "create_scope"
	ReadScope   = "read_scope"
	UpdateScope = "update_scope"
	DeleteScope = "delete_scope"

	// Definition permissions
	CreateDefinition = "create_definition"
	ReadDefinition   = "read_definition"
	UpdateDefinition = "update_definition"
	DeleteDefinition = "delete_definition"
	// Approving or rejecting definitions
	ReviewDefinition = "review_definition"

	// Translation server permissions
	CreateTranslationServer = "create_translation_server"
	ReadTranslationServer   = "read_translation_server"
	UpdateTranslationServer = "update_translation_server"
	DeleteTranslationServer = "delete_translation_server"

	// Requesting machine translations, which loads the translation servers.
	UseMachineTranslation = "use_machine_translation"
)

// AllPermissions returns all defined permissions
func AllPermissions() []string {
	return []string{
		CreateLocale,
		ReadLocale,
		UpdateLocale,
		DeleteLocale,
		PermanentDeleteLocale,
		CreateEntry,
		ReadEntry,
		UpdateEntry,
		DeleteEntry,
		ReviewEntry,
		ImportEntry,
		ExportEntry,
		CreateUser,
		ReadUser,
		UpdateUser,
		DeleteUser,
		CreateRole,
		ReadRole,
		UpdateRole,
		DeleteRole,
		ReadPermission,
		CreateTerminology,
		ReadTerminology,
		UpdateTerminology,
		DeleteTerminology,
		CreateScope,
		ReadScope,
		UpdateScope,
		DeleteScope,
		CreateDefinition,
		ReadDefinition,
		UpdateDefinition,
		DeleteDefinition,
		ReviewDefinition,
		CreateTranslationServer,
		ReadTranslationServer,
		UpdateTranslationServer,
		DeleteTranslationServer,
		UseMachineTranslation,
	}
}

// GrantedWith lists, for permissions split off from broader ones, the permissions a
// role needed for the same thing before. When such a permission is added to an
// existing database, the roles holding all of them get it, so nobody loses access.
var GrantedWith = map[string][]string{
	ImportEntry: {CreateEntry, UpdateEntry},
	ExportEntry: {ReadEntry},
}
