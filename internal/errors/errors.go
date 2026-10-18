package errors

import (
	"fmt"

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
	ErrLocaleNotFound           = "LOCALE_NOT_FOUND"
	ErrLocaleAlreadyExists      = "LOCALE_ALREADY_EXISTS"
	ErrLocaleDefaultDelete      = "LOCALE_DEFAULT_DELETE"
	ErrLocaleDefaultDestroy     = "LOCALE_DEFAULT_DESTROY"
	ErrLocaleMustBeSoftDeleted  = "LOCALE_MUST_BE_SOFT_DELETED"
	ErrLocaleInsufficientAccess = "LOCALE_INSUFFICIENT_ACCESS"

	// User-specific errors
	ErrUserNotFound       = "USER_NOT_FOUND"
	ErrUserAlreadyExists  = "USER_ALREADY_EXISTS"
	ErrInvalidCredentials = "INVALID_CREDENTIALS"

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
