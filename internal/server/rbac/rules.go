package rbac

import (
	"context"

	"connectrpc.com/connect"

	"github.com/kazuto/parlance-server/gen/parlance/v1/parlancev1connect"
	"github.com/kazuto/parlance-server/internal/database"
	"github.com/kazuto/parlance-server/internal/middleware"
	"github.com/kazuto/parlance-server/internal/permissions"
)

// Rules lists the permissions each protected procedure needs; the user must hold all of
// them. REPLACE_ALL imports also need delete_entry, and approving imports review_entry,
// which the handler checks.
var Rules = map[string][]string{
	// ActivityService
	parlancev1connect.ActivityServiceListRecentActivityProcedure: {permissions.ReadEntry},

	// DefinitionService
	parlancev1connect.DefinitionServiceCreateDefinitionProcedure:  {permissions.CreateDefinition},
	parlancev1connect.DefinitionServiceDeleteDefinitionProcedure:  {permissions.DeleteDefinition},
	parlancev1connect.DefinitionServiceGetDefinitionProcedure:     {permissions.ReadDefinition},
	parlancev1connect.DefinitionServiceListDefinitionsProcedure:   {permissions.ReadDefinition},
	parlancev1connect.DefinitionServiceReviewDefinitionsProcedure: {permissions.ReviewDefinition},
	parlancev1connect.DefinitionServiceRestoreDefinitionProcedure: {permissions.DeleteDefinition},
	parlancev1connect.DefinitionServiceUpdateDefinitionProcedure:  {permissions.UpdateDefinition},

	// EntryService
	parlancev1connect.EntryServiceCreateEntryProcedure:         {permissions.CreateEntry},
	parlancev1connect.EntryServiceDeleteEntryProcedure:         {permissions.DeleteEntry},
	parlancev1connect.EntryServiceGetEntryProcedure:            {permissions.ReadEntry},
	parlancev1connect.EntryServiceGetEntryHistoryProcedure:     {permissions.ReadEntry},
	parlancev1connect.EntryServiceListEntriesProcedure:         {permissions.ReadEntry},
	parlancev1connect.EntryServiceRestoreEntryProcedure:        {permissions.DeleteEntry},
	parlancev1connect.EntryServiceSearchEntriesProcedure:       {permissions.ReadEntry},
	parlancev1connect.EntryServiceUpdateEntriesScopesProcedure: {permissions.UpdateEntry},
	parlancev1connect.EntryServiceUpdateEntryProcedure:         {permissions.UpdateEntry},

	// ExportService
	parlancev1connect.ExportServiceExportJsonProcedure: {permissions.ExportEntry},
	parlancev1connect.ExportServiceExportPhpProcedure:  {permissions.ExportEntry},
	parlancev1connect.ExportServiceExportPoProcedure:   {permissions.ExportEntry},
	parlancev1connect.ExportServiceExportZipProcedure:  {permissions.ExportEntry},

	// ImportService
	parlancev1connect.ImportServiceImportJsonProcedure: {permissions.ImportEntry},
	parlancev1connect.ImportServiceImportPhpProcedure:  {permissions.ImportEntry},
	parlancev1connect.ImportServiceImportPoProcedure:   {permissions.ImportEntry},
	parlancev1connect.ImportServiceImportZipProcedure:  {permissions.ImportEntry},

	// LocaleService
	parlancev1connect.LocaleServiceCreateLocaleProcedure:     {permissions.CreateLocale},
	parlancev1connect.LocaleServiceDeleteLocaleProcedure:     {permissions.DeleteLocale},
	parlancev1connect.LocaleServiceDestroyLocaleProcedure:    {permissions.PermanentDeleteLocale},
	parlancev1connect.LocaleServiceGetDefaultLocaleProcedure: {permissions.ReadLocale},
	parlancev1connect.LocaleServiceGetLocaleProcedure:        {permissions.ReadLocale},
	parlancev1connect.LocaleServiceListLocalesProcedure:      {permissions.ReadLocale},
	parlancev1connect.LocaleServiceRestoreLocaleProcedure:    {permissions.DeleteLocale},
	parlancev1connect.LocaleServiceSetDefaultLocaleProcedure: {permissions.UpdateLocale},
	parlancev1connect.LocaleServiceUpdateLocaleProcedure:     {permissions.UpdateLocale},

	// LocalizationService
	parlancev1connect.LocalizationServiceBatchTranslateProcedure:      {permissions.UpdateEntry, permissions.UseMachineTranslation},
	parlancev1connect.LocalizationServiceCreateLocalizationProcedure:  {permissions.UpdateEntry},
	parlancev1connect.LocalizationServiceDeleteLocalizationProcedure:  {permissions.UpdateEntry},
	parlancev1connect.LocalizationServiceGetLocalizationProcedure:     {permissions.ReadEntry},
	parlancev1connect.LocalizationServiceListLocalizationsProcedure:   {permissions.ReadEntry},
	parlancev1connect.LocalizationServiceReviewLocalizationsProcedure: {permissions.ReviewEntry},
	parlancev1connect.LocalizationServiceRestoreLocalizationProcedure: {permissions.UpdateEntry},
	parlancev1connect.LocalizationServiceSuggestTranslationsProcedure: {permissions.UpdateEntry, permissions.UseMachineTranslation},
	parlancev1connect.LocalizationServiceUpdateLocalizationProcedure:  {permissions.UpdateEntry},

	// PermissionService
	parlancev1connect.PermissionServiceGetPermissionProcedure:   {permissions.ReadPermission},
	parlancev1connect.PermissionServiceListPermissionsProcedure: {permissions.ReadPermission},

	// RoleService
	parlancev1connect.RoleServiceCreateRoleProcedure:  {permissions.CreateRole},
	parlancev1connect.RoleServiceDeleteRoleProcedure:  {permissions.DeleteRole},
	parlancev1connect.RoleServiceGetRoleProcedure:     {permissions.ReadRole},
	parlancev1connect.RoleServiceListRolesProcedure:   {permissions.ReadRole},
	parlancev1connect.RoleServiceRestoreRoleProcedure: {permissions.DeleteRole},
	parlancev1connect.RoleServiceUpdateRoleProcedure:  {permissions.UpdateRole},

	// ScopeService
	parlancev1connect.ScopeServiceAddScopeToEntryProcedure:      {permissions.UpdateEntry},
	parlancev1connect.ScopeServiceCreateScopeProcedure:          {permissions.CreateScope},
	parlancev1connect.ScopeServiceDeleteScopeProcedure:          {permissions.DeleteScope},
	parlancev1connect.ScopeServiceGetEntryScopesProcedure:       {permissions.ReadScope},
	parlancev1connect.ScopeServiceGetScopeProcedure:             {permissions.ReadScope},
	parlancev1connect.ScopeServiceListScopesProcedure:           {permissions.ReadScope},
	parlancev1connect.ScopeServiceRemoveScopeFromEntryProcedure: {permissions.UpdateEntry},
	parlancev1connect.ScopeServiceRestoreScopeProcedure:         {permissions.DeleteScope},
	parlancev1connect.ScopeServiceUpdateScopeProcedure:          {permissions.UpdateScope},

	// StatsService
	parlancev1connect.StatsServiceGetStatsProcedure: {permissions.ReadEntry},

	// TaskService
	parlancev1connect.TaskServiceCancelTaskRunProcedure:      {permissions.RunTask},
	parlancev1connect.TaskServiceCheckTaskAccessProcedure:    {permissions.UpdateTask},
	parlancev1connect.TaskServiceCreateTaskProcedure:         {permissions.CreateTask},
	parlancev1connect.TaskServiceDeleteTaskProcedure:         {permissions.DeleteTask},
	parlancev1connect.TaskServiceGetTaskProcedure:            {permissions.ReadTask},
	parlancev1connect.TaskServiceGetTaskRunProcedure:         {permissions.ReadTask},
	parlancev1connect.TaskServiceGetTaskSettingsProcedure:    {permissions.ReadTask},
	parlancev1connect.TaskServiceListTaskRunsProcedure:       {permissions.ReadTask},
	parlancev1connect.TaskServiceListTasksProcedure:          {permissions.ReadTask},
	parlancev1connect.TaskServiceRunTaskProcedure:            {permissions.RunTask},
	parlancev1connect.TaskServiceUpdateTaskProcedure:         {permissions.UpdateTask},
	parlancev1connect.TaskServiceUpdateTaskSettingsProcedure: {permissions.UpdateTask},

	// TerminologyService
	parlancev1connect.TerminologyServiceCreateTerminologyProcedure:    {permissions.CreateTerminology},
	parlancev1connect.TerminologyServiceDeleteTerminologyProcedure:    {permissions.DeleteTerminology},
	parlancev1connect.TerminologyServiceGetDefinitionHistoryProcedure: {permissions.ReadDefinition},
	parlancev1connect.TerminologyServiceGetTerminologyProcedure:       {permissions.ReadTerminology},
	parlancev1connect.TerminologyServiceListTerminologiesProcedure:    {permissions.ReadTerminology},
	parlancev1connect.TerminologyServiceRestoreTerminologyProcedure:   {permissions.DeleteTerminology},
	parlancev1connect.TerminologyServiceSuggestTerminologiesProcedure: {permissions.ReadTerminology},
	parlancev1connect.TerminologyServiceUpdateTerminologyProcedure:    {permissions.UpdateTerminology},

	// TranslationServerService
	parlancev1connect.TranslationServerServiceCreateTranslationServerProcedure: {permissions.CreateTranslationServer},
	parlancev1connect.TranslationServerServiceDeleteTranslationServerProcedure: {permissions.DeleteTranslationServer},
	parlancev1connect.TranslationServerServiceListTranslationServersProcedure:  {permissions.ReadTranslationServer},
	parlancev1connect.TranslationServerServiceTestTranslationServerProcedure:   {permissions.UpdateTranslationServer},
	parlancev1connect.TranslationServerServiceUpdateTranslationServerProcedure: {permissions.UpdateTranslationServer},

	// UserService
	parlancev1connect.UserServiceAssignRoleProcedure:  {permissions.UpdateUser},
	parlancev1connect.UserServiceCreateUserProcedure:  {permissions.CreateUser},
	parlancev1connect.UserServiceDeleteUserProcedure:  {permissions.DeleteUser},
	parlancev1connect.UserServiceGetUserProcedure:     {permissions.ReadUser},
	parlancev1connect.UserServiceListUsersProcedure:   {permissions.ReadUser},
	parlancev1connect.UserServiceRestoreUserProcedure: {permissions.DeleteUser},
	parlancev1connect.UserServiceUpdateUserProcedure:  {permissions.UpdateUser},

	// WebhookService
	parlancev1connect.WebhookServiceCreateWebhookProcedure:            {permissions.CreateWebhook},
	parlancev1connect.WebhookServiceDeleteWebhookProcedure:            {permissions.DeleteWebhook},
	parlancev1connect.WebhookServiceGetWebhookDeliveryProcedure:       {permissions.ReadWebhook},
	parlancev1connect.WebhookServiceListWebhookDeliveriesProcedure:    {permissions.ReadWebhook},
	parlancev1connect.WebhookServiceListWebhooksProcedure:             {permissions.ReadWebhook},
	parlancev1connect.WebhookServiceRedeliverWebhookDeliveryProcedure: {permissions.UpdateWebhook},
	parlancev1connect.WebhookServiceTestWebhookProcedure:              {permissions.UpdateWebhook},
	parlancev1connect.WebhookServiceUpdateWebhookProcedure:            {permissions.UpdateWebhook},
}

// Interceptor enforces Rules on the services it is installed on
func Interceptor(db *database.DB) connect.UnaryInterceptorFunc {
	return middleware.NewRBACInterceptor(db, Rules)
}

// Authorize checks the Rules for a procedure as the interceptor does
func Authorize(ctx context.Context, db *database.DB, procedure string) (context.Context, error) {
	return middleware.Authorize(ctx, db, Rules, procedure)
}
