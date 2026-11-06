package errors

import (
	"fmt"
	"log"

	"connectrpc.com/connect"
)

// Error codes - single source of truth for all error codes
const (
	// General errors
	ErrInvalidInput       = "INVALID_INPUT"
	ErrNotFound           = "NOT_FOUND"
	ErrAlreadyExists      = "ALREADY_EXISTS"
	ErrInternal           = "INTERNAL_ERROR"
	ErrUnauthenticated    = "UNAUTHENTICATED"
	ErrPermissionDenied   = "PERMISSION_DENIED"
	ErrFailedPrecondition = "FAILED_PRECONDITION"

	// Locale-specific errors
	ErrLocaleNotFound          = "LOCALE_NOT_FOUND"
	ErrLocaleAlreadyExists     = "LOCALE_ALREADY_EXISTS"
	ErrLocaleDefaultDelete     = "LOCALE_DEFAULT_DELETE"
	ErrLocaleDefaultDestroy    = "LOCALE_DEFAULT_DESTROY"
	ErrLocaleMustBeSoftDeleted = "LOCALE_MUST_BE_SOFT_DELETED"

	// User-specific errors
	ErrUserNotFound       = "USER_NOT_FOUND"
	ErrUserAlreadyExists  = "USER_ALREADY_EXISTS"
	ErrInvalidCredentials = "INVALID_CREDENTIALS"
	// Changing one's password needs the current one.
	ErrWrongPassword = "WRONG_PASSWORD"

	// Setup errors
	ErrSetupAlreadyCompleted = "SETUP_ALREADY_COMPLETED"
	ErrSetupFailed           = "SETUP_FAILED"

	// Entry-specific errors
	ErrEntryNotFound      = "ENTRY_NOT_FOUND"
	ErrEntryAlreadyExists = "ENTRY_ALREADY_EXISTS"

	// Role-specific errors
	ErrRoleNotFound      = "ROLE_NOT_FOUND"
	ErrRoleAlreadyExists = "ROLE_ALREADY_EXISTS"

	// Permission-specific errors
	ErrPermissionNotFound = "PERMISSION_NOT_FOUND"

	// Terminology-specific errors
	ErrTerminologyNotFound      = "TERMINOLOGY_NOT_FOUND"
	ErrTerminologyAlreadyExists = "TERMINOLOGY_ALREADY_EXISTS"

	// Definition-specific errors
	ErrDefinitionNotFound      = "DEFINITION_NOT_FOUND"
	ErrDefinitionAlreadyExists = "DEFINITION_ALREADY_EXISTS"

	// Scope-specific errors
	ErrScopeNotFound      = "SCOPE_NOT_FOUND"
	ErrScopeAlreadyExists = "SCOPE_ALREADY_EXISTS"

	// Localization-specific errors
	ErrLocalizationNotFound = "LOCALIZATION_NOT_FOUND"

	// Translation server errors
	ErrTranslationServerNotFound = "TRANSLATION_SERVER_NOT_FOUND"

	// Task errors
	ErrTaskNotFound    = "TASK_NOT_FOUND"
	ErrTaskRunNotFound = "TASK_RUN_NOT_FOUND"

	// Webhook errors
	ErrWebhookNotFound         = "WEBHOOK_NOT_FOUND"
	ErrWebhookDeliveryNotFound = "WEBHOOK_DELIVERY_NOT_FOUND"

	// Single sign-on errors
	ErrSSOProviderNotFound = "SSO_PROVIDER_NOT_FOUND"
	// The sign-in expired, or its code was used already.
	ErrSSOLoginExpired = "SSO_LOGIN_EXPIRED"
	// The provider turned the sign-in down, or its ID token isn't valid.
	ErrSSOFailed          = "SSO_FAILED"
	ErrSSONoEmail         = "SSO_NO_EMAIL"
	ErrSSOEmailUnverified = "SSO_EMAIL_UNVERIFIED"
	// Nobody has the email, and the provider creates no accounts.
	ErrSSONoAccount      = "SSO_NO_ACCOUNT"
	ErrSSOAccountDeleted = "SSO_ACCOUNT_DELETED"
	// Saving an API key needs ENCRYPTION_KEY.
	ErrEncryptionKeyMissing = "ENCRYPTION_KEY_MISSING"

	// API token errors
	ErrAPITokenNotFound = "API_TOKEN_NOT_FOUND"
	// A user has as many API tokens as they may.
	ErrAPITokenLimit = "API_TOKEN_LIMIT"
	// Managing API tokens and changing how one signs in need a sign-in, not an API token.
	ErrAPITokenNotAllowed = "API_TOKEN_NOT_ALLOWED"
	// An admin turned registration off.
	ErrRegistrationClosed = "REGISTRATION_CLOSED"
)

// ErrorWithCode creates a Connect error with an error code in the message
func ErrorWithCode(code connect.Code, errorCode string, message string) error {
	return connect.NewError(code, fmt.Errorf("[%s] %s", errorCode, message))
}

// InvalidInput returns an invalid argument error
func InvalidInput(errorCode string, message string) error {
	return ErrorWithCode(connect.CodeInvalidArgument, errorCode, message)
}

// NotFound returns a not found error
func NotFound(errorCode string, message string) error {
	return ErrorWithCode(connect.CodeNotFound, errorCode, message)
}

// AlreadyExists returns an already exists error
func AlreadyExists(errorCode string, message string) error {
	return ErrorWithCode(connect.CodeAlreadyExists, errorCode, message)
}

// Internal returns an internal error
func Internal(errorCode string, message string) error {
	return ErrorWithCode(connect.CodeInternal, errorCode, message)
}

// InternalCause logs the underlying error and returns an internal error carrying only the
// generic message, so database details never reach clients.
func InternalCause(cause error, message string) error {
	log.Printf("%s: %v", message, cause)

	return Internal(ErrInternal, message)
}

// Unauthenticated returns an unauthenticated error
func Unauthenticated(errorCode string, message string) error {
	return ErrorWithCode(connect.CodeUnauthenticated, errorCode, message)
}

// PermissionDenied returns a permission denied error
func PermissionDenied(errorCode string, message string) error {
	return ErrorWithCode(connect.CodePermissionDenied, errorCode, message)
}

// FailedPrecondition returns a failed precondition error
func FailedPrecondition(errorCode string, message string) error {
	return ErrorWithCode(connect.CodeFailedPrecondition, errorCode, message)
}
