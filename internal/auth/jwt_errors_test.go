package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateTokenErrors(t *testing.T) {
	secret := "test-secret"

	tests := []struct {
		name        string
		tokenString string
		secret      string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty token string",
			tokenString: "",
			secret:      secret,
			wantErr:     true,
			errContains: "token",
		},
		{
			name:        "malformed token",
			tokenString: "not.a.valid.token.format",
			secret:      secret,
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "token with invalid format",
			tokenString: "invalid",
			secret:      secret,
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "token with only two parts",
			tokenString: "header.payload",
			secret:      secret,
			wantErr:     true,
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := ValidateToken(tt.tokenString, tt.secret)

			if tt.wantErr {
				if err == nil {
					t.Error("ValidateToken() expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateToken() error = %v, should contain %v", err, tt.errContains)
				}
				if claims != nil {
					t.Error("ValidateToken() expected nil claims on error")
				}
			} else {
				if err != nil {
					t.Errorf("ValidateToken() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateTokenWithWrongSigningMethod(t *testing.T) {
	userID := "test-user"
	email := "test@example.com"

	// Create token with RS256 (wrong method, we expect HS256)
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Try to create a token string that looks valid but will fail validation
	// Note: We can't actually use RS256 without keys, so we'll test with a token
	// signed with a different HMAC secret
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("different-secret"))

	// Try to validate with correct secret - should fail
	_, err := ValidateToken(tokenString, "test-secret")
	if err == nil {
		t.Error("ValidateToken() expected error for token with wrong signing secret, got nil")
	}
}

func TestValidateTokenInvalidClaims(t *testing.T) {
	secret := "test-secret"

	// Create a token with standard claims (not our custom Claims type)
	standardClaims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, standardClaims)
	tokenString, _ := token.SignedString([]byte(secret))

	// Try to parse as our Claims type - should work but claims will be empty
	claims, err := ValidateToken(tokenString, secret)
	if err != nil {
		t.Errorf("ValidateToken() error = %v, want nil (should parse but have empty custom fields)", err)
	}

	// Claims should exist but custom fields should be empty
	if claims.UserID != "" {
		t.Errorf("UserID = %v, want empty", claims.UserID)
	}
	if claims.Email != "" {
		t.Errorf("Email = %v, want empty", claims.Email)
	}
}

func TestGenerateTokenPairWithEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		email         string
		secret        string
		accessExpiry  time.Duration
		refreshExpiry time.Duration
		wantErr       bool
	}{
		{
			name:          "empty user ID",
			userID:        "",
			email:         "test@example.com",
			secret:        "secret",
			accessExpiry:  15 * time.Minute,
			refreshExpiry: 7 * 24 * time.Hour,
			wantErr:       false, // Should still work, just empty claims
		},
		{
			name:          "empty email",
			userID:        "user-123",
			email:         "",
			secret:        "secret",
			accessExpiry:  15 * time.Minute,
			refreshExpiry: 7 * 24 * time.Hour,
			wantErr:       false, // Should still work, just empty email
		},
		{
			name:          "empty secret",
			userID:        "user-123",
			email:         "test@example.com",
			secret:        "",
			accessExpiry:  15 * time.Minute,
			refreshExpiry: 7 * 24 * time.Hour,
			wantErr:       false, // JWT library accepts empty secret
		},
		{
			name:          "zero expiry",
			userID:        "user-123",
			email:         "test@example.com",
			secret:        "secret",
			accessExpiry:  0,
			refreshExpiry: 0,
			wantErr:       false, // Should work but tokens expire immediately
		},
		{
			name:          "negative expiry",
			userID:        "user-123",
			email:         "test@example.com",
			secret:        "secret",
			accessExpiry:  -1 * time.Hour,
			refreshExpiry: -1 * time.Hour,
			wantErr:       false, // Should work but tokens already expired
		},
		{
			name:          "very long expiry",
			userID:        "user-123",
			email:         "test@example.com",
			secret:        "secret",
			accessExpiry:  365 * 24 * time.Hour,
			refreshExpiry: 10 * 365 * 24 * time.Hour,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := GenerateTokenPair(tt.userID, tt.email, tt.secret, tt.accessExpiry, tt.refreshExpiry)

			if tt.wantErr {
				if err == nil {
					t.Error("GenerateTokenPair() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GenerateTokenPair() unexpected error = %v", err)
				return
			}

			if tokens == nil {
				t.Fatal("GenerateTokenPair() returned nil tokens")
			}

			if tokens.AccessToken == "" {
				t.Error("AccessToken is empty")
			}

			if tokens.RefreshToken == "" {
				t.Error("RefreshToken is empty")
			}

			// Verify tokens can be validated
			claims, err := ValidateToken(tokens.AccessToken, tt.secret)
			if err != nil && tt.accessExpiry > 0 {
				t.Errorf("ValidateToken(access) error = %v", err)
			}

			if tt.accessExpiry > 0 && claims != nil {
				if claims.UserID != tt.userID {
					t.Errorf("UserID = %v, want %v", claims.UserID, tt.userID)
				}
				if claims.Email != tt.email {
					t.Errorf("Email = %v, want %v", claims.Email, tt.email)
				}
			}
		})
	}
}

func TestValidateTokenNotYetValid(t *testing.T) {
	// This test verifies the NotBefore claim
	// In our implementation, NotBefore is set to Now(), so token is always immediately valid
	// But we test that the claim is set correctly

	secret := "test-secret"
	userID := "test-user"
	email := "test@example.com"

	tokens, err := GenerateTokenPair(userID, email, secret, 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := ValidateToken(tokens.AccessToken, secret)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	// Verify NotBefore is set and is in the past or present
	if claims.NotBefore == nil {
		t.Error("NotBefore claim is not set")
	} else if claims.NotBefore.After(time.Now()) {
		t.Error("NotBefore is in the future - token should not be valid yet")
	}

	// Verify IssuedAt is set
	if claims.IssuedAt == nil {
		t.Error("IssuedAt claim is not set")
	}

	// Verify ExpiresAt is set and is in the future
	if claims.ExpiresAt == nil {
		t.Error("ExpiresAt claim is not set")
	} else if claims.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt is in the past - token is already expired")
	}
}
