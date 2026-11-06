package middleware

import (
	"context"
	stderrors "errors"
	"log"
	"net/http"
	"strings"

	"github.com/kazuto/parlance-server/internal/apitoken"
	"github.com/kazuto/parlance-server/internal/auth"
	"github.com/kazuto/parlance-server/internal/errors"
)

// ContextKey is a custom type for context keys to avoid collisions
type ContextKey string

// ContextKeyUserID is the context key for user ID
const ContextKeyUserID ContextKey = "user_id"

// UserID returns the signed-in user's ID, or "" without one
func UserID(ctx context.Context) string {
	userID, _ := ctx.Value(ContextKeyUserID).(string)

	return userID
}

// UserIDPtr returns the signed-in user's ID for created_by/updated_by columns, or nil without one
func UserIDPtr(ctx context.Context) *string {
	if userID := UserID(ctx); userID != "" {
		return &userID
	}

	return nil
}

// ContextKeyAPIToken is the context key for the ID of the API token a request was made with
const ContextKeyAPIToken ContextKey = "api_token_id"

// TokenAuthenticator finds the user an API token belongs to, see apitoken.Authenticator
type TokenAuthenticator interface {
	Authenticate(ctx context.Context, secret string) (tokenID, userID string, err error)
}

// ViaAPIToken tells whether the request was made with an API token rather than by signing in
func ViaAPIToken(ctx context.Context) bool {
	tokenID, _ := ctx.Value(ContextKeyAPIToken).(string)

	return tokenID != ""
}

// RequireSignIn turns down a request made with an API token, for what a token must not do:
// manage API tokens, or change how its user signs in
func RequireSignIn(ctx context.Context) error {
	if ViaAPIToken(ctx) {
		return errors.PermissionDenied(errors.ErrAPITokenNotAllowed, "this needs a sign-in, not an API token")
	}

	return nil
}

// authFailure says why a request couldn't be authenticated
type authFailure struct {
	status  int
	code    string
	message string
}

func unauthenticated(message string) *authFailure {
	return &authFailure{status: http.StatusUnauthorized, code: "unauthenticated", message: message}
}

// authenticate validates the request's bearer token, a sign-in token or an API token, and
// returns the request with the user in its context. A request with a user already, from
// an earlier middleware, is returned as it is.
func authenticate(r *http.Request, secret string, tokens TokenAuthenticator) (*http.Request, *authFailure) {
	if UserID(r.Context()) != "" {
		return r, nil
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return r, unauthenticated("authorization header required")
	}

	token, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok || token == "" || strings.Contains(token, " ") {
		return r, unauthenticated("invalid authorization header format")
	}

	if apitoken.IsToken(token) {
		if tokens == nil {
			return r, unauthenticated("invalid or expired token")
		}

		tokenID, userID, err := tokens.Authenticate(r.Context(), token)
		if stderrors.Is(err, apitoken.ErrInvalid) {
			return r, unauthenticated("invalid or expired token")
		}

		if err != nil {
			log.Printf("checking an API token failed: %v", err)

			return r, &authFailure{status: http.StatusInternalServerError, code: "internal", message: "could not check the token"}
		}

		ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)

		return r.WithContext(context.WithValue(ctx, ContextKeyAPIToken, tokenID)), nil
	}

	claims, err := auth.ValidateAccessToken(token, secret)
	if err != nil {
		return r, unauthenticated("invalid or expired token")
	}

	return r.WithContext(context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)), nil
}

// AuthMiddleware adds the user of a valid sign-in or API token to the context, and lets
// requests without one through
func AuthMiddleware(secret string, tokens TokenAuthenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r, _ = authenticate(r, secret, tokens)
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth is a stricter middleware that rejects requests without valid auth
func RequireAuth(secret string, tokens TokenAuthenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r, failure := authenticate(r, secret, tokens)
			if failure != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(failure.status)
				_, _ = w.Write([]byte(`{"code":"` + failure.code + `","message":"` + failure.message + `"}`))

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
