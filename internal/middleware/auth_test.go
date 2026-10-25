package middleware

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/kazuto/parlance-server/internal/apitoken"
	"github.com/kazuto/parlance-server/internal/auth"
)

func TestAuthMiddleware(t *testing.T) {
	secret := "test-secret"
	userID := "test-user-id"
	email := "test@example.com"

	// Generate a valid token
	tokens, err := auth.GenerateTokenPair(userID, email, secret, 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	tests := []struct {
		name           string
		authHeader     string
		expectUserID   bool
		expectedUserID string
	}{
		{
			name:           "valid token",
			authHeader:     "Bearer " + tokens.AccessToken,
			expectUserID:   true,
			expectedUserID: userID,
		},
		{
			name:         "no auth header",
			authHeader:   "",
			expectUserID: false,
		},
		{
			name:         "invalid format - no Bearer",
			authHeader:   tokens.AccessToken,
			expectUserID: false,
		},
		{
			name:         "invalid token",
			authHeader:   "Bearer invalid.token.here",
			expectUserID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that checks context
			var contextUserID string

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if uid, ok := r.Context().Value(ContextKeyUserID).(string); ok {
					contextUserID = uid
				}

				w.WriteHeader(http.StatusOK)
			})

			// Wrap with auth middleware
			middleware := AuthMiddleware(secret, nil)
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Record response
			rec := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rec, req)

			// Check response
			if rec.Code != http.StatusOK {
				t.Errorf("Status code = %v, want %v", rec.Code, http.StatusOK)
			}

			// Check context
			if tt.expectUserID {
				if contextUserID != tt.expectedUserID {
					t.Errorf("Context user_id = %v, want %v", contextUserID, tt.expectedUserID)
				}
			} else {
				if contextUserID != "" {
					t.Errorf("Context user_id = %v, want empty", contextUserID)
				}
			}
		})
	}
}

func TestRequireAuth(t *testing.T) {
	secret := "test-secret"
	userID := "test-user-id"
	email := "test@example.com"

	// Generate a valid token
	tokens, err := auth.GenerateTokenPair(userID, email, secret, 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectUserID   bool
	}{
		{
			name:           "valid token",
			authHeader:     "Bearer " + tokens.AccessToken,
			expectedStatus: http.StatusOK,
			expectUserID:   true,
		},
		{
			name:           "no auth header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
		{
			name:           "invalid format",
			authHeader:     tokens.AccessToken,
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid.token.here",
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var handlerCalled bool

			var contextUserID string

			// Create a test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true

				if uid, ok := r.Context().Value(ContextKeyUserID).(string); ok {
					contextUserID = uid
				}

				w.WriteHeader(http.StatusOK)
			})

			// Wrap with RequireAuth
			middleware := RequireAuth(secret, nil)
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Record response
			rec := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rec, req)

			// Check status
			if rec.Code != tt.expectedStatus {
				t.Errorf("Status code = %v, want %v", rec.Code, tt.expectedStatus)
			}

			// Check if handler was called
			if tt.expectUserID {
				if !handlerCalled {
					t.Error("Handler was not called with valid token")
				}

				if contextUserID != userID {
					t.Errorf("Context user_id = %v, want %v", contextUserID, userID)
				}
			} else {
				if handlerCalled {
					t.Error("Handler was called despite invalid/missing token")
				}
			}
		})
	}
}

func TestRequireAuthWithExpiredToken(t *testing.T) {
	secret := "test-secret"
	userID := "test-user-id"
	email := "test@example.com"

	// Generate token with very short expiry
	tokens, err := auth.GenerateTokenPair(userID, email, secret, 1*time.Millisecond, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireAuth(secret, nil)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)

	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status code with expired token = %v, want %v", rec.Code, http.StatusUnauthorized)
	}
}

func TestExtractUserIDFromContext(t *testing.T) {
	ctx := context.Background()
	userID := "test-user-123"

	// Test with user_id in context
	ctxWithUser := context.WithValue(ctx, ContextKeyUserID, userID)

	extractedID, ok := ctxWithUser.Value(ContextKeyUserID).(string)
	if !ok {
		t.Fatal("Failed to extract user_id from context")
	}

	if extractedID != userID {
		t.Errorf("Extracted user_id = %v, want %v", extractedID, userID)
	}

	// Test without user_id in context
	_, ok = ctx.Value(ContextKeyUserID).(string)
	if ok {
		t.Error("Should not find user_id in empty context")
	}
}

// fakeTokens knows the API token "plc_good", and fails with err when set
type fakeTokens struct {
	calls int
	err   error
}

func (f *fakeTokens) Authenticate(_ context.Context, secret string) (string, string, error) {
	f.calls++

	switch {
	case f.err != nil:
		return "", "", f.err
	case secret != "plc_good":
		return "", "", apitoken.ErrInvalid
	}

	return "token-1", "user-1", nil
}

func TestAPITokens(t *testing.T) {
	secret := "test-secret"

	signIn, err := auth.GenerateTokenPair("user-2", "user@example.com", secret, 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		bearer   string
		tokens   *fakeTokens
		status   int
		userID   string
		viaToken bool
	}{
		{"API token", "plc_good", &fakeTokens{}, http.StatusOK, "user-1", true},
		{"sign-in token", signIn.AccessToken, &fakeTokens{}, http.StatusOK, "user-2", false},
		{"unknown API token", "plc_bad", &fakeTokens{}, http.StatusUnauthorized, "", false},
		{"failing check", "plc_good", &fakeTokens{err: stderrors.New("database down")}, http.StatusInternalServerError, "", false},
		{"API token without an authenticator", "plc_good", nil, http.StatusUnauthorized, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userID string

			var viaToken bool

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				userID = UserID(r.Context())
				viaToken = ViaAPIToken(r.Context())
			})

			var tokens TokenAuthenticator
			if tt.tokens != nil {
				tokens = tt.tokens
			}

			// As the server chains them: the optional check for every request, then the
			// required one for protected services.
			chain := AuthMiddleware(secret, tokens)(RequireAuth(secret, tokens)(handler))

			req := httptest.NewRequest("POST", "/parlance.v1.ExportService/ExportJson", nil)
			req.Header.Set("Authorization", "Bearer "+tt.bearer)

			rec := httptest.NewRecorder()
			chain.ServeHTTP(rec, req)

			if rec.Code != tt.status || userID != tt.userID || viaToken != tt.viaToken {
				t.Errorf("status %d, user %q, via token %v; want %d, %q, %v", rec.Code, userID, viaToken, tt.status, tt.userID, tt.viaToken)
			}

			// The required check reuses the optional one's user.
			if tt.tokens != nil && tt.status == http.StatusOK && tt.viaToken && tt.tokens.calls != 1 {
				t.Errorf("token checked %d times, want once", tt.tokens.calls)
			}
		})
	}
}

func TestRequireSignIn(t *testing.T) {
	signedIn := context.WithValue(context.Background(), ContextKeyUserID, "user-1")

	if err := RequireSignIn(signedIn); err != nil {
		t.Errorf("signed in: %v", err)
	}

	err := RequireSignIn(context.WithValue(signedIn, ContextKeyAPIToken, "token-1"))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("with an API token: %v, want permission denied", err)
	}
}
