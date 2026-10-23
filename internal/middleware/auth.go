package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/kazuto/parlance-server/internal/auth"
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

// authenticate validates the request's bearer token and returns the request with the user in
// its context. The message describes why authentication failed.
func authenticate(r *http.Request, secret string) (*http.Request, string) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return r, "authorization header required"
	}

	token, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok || token == "" || strings.Contains(token, " ") {
		return r, "invalid authorization header format"
	}

	claims, err := auth.ValidateAccessToken(token, secret)
	if err != nil {
		return r, "invalid or expired token"
	}

	return r.WithContext(context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)), ""
}

// AuthMiddleware validates JWT tokens and adds user info to context
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r, _ = authenticate(r, secret)
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth is a stricter middleware that rejects requests without valid auth
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r, failure := authenticate(r, secret)
			if failure != "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":"unauthenticated","message":"` + failure + `"}`))

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
