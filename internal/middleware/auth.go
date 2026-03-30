package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/kazuto/parlance-server/internal/auth"
)

// AuthMiddleware validates JWT tokens and adds user info to context
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader != "" {
				// Extract token from "Bearer <token>" format
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					token := parts[1]

					// Validate token
					claims, err := auth.ValidateToken(token, secret)
					if err == nil {
						// Add user info to context
						ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
						ctx = context.WithValue(ctx, "user_email", claims.Email)
						r = r.WithContext(ctx)
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth is a stricter middleware that rejects requests without valid auth
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"code":"unauthenticated","message":"authorization header required"}`))

				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"code":"unauthenticated","message":"invalid authorization header format"}`))

				return
			}

			token := parts[1]
			claims, err := auth.ValidateToken(token, secret)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"code":"unauthenticated","message":"invalid or expired token"}`))

				return
			}

			ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
			ctx = context.WithValue(ctx, "user_email", claims.Email)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
