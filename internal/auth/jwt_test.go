package auth

import (
	"testing"
	"time"
)

func TestGenerateTokenPair(t *testing.T) {
	userID := "test-user-id"
	email := "test@example.com"
	secret := "test-secret-key"
	accessExpiry := 15 * time.Minute
	refreshExpiry := 7 * 24 * time.Hour

	tokens, err := GenerateTokenPair(userID, email, secret, accessExpiry, refreshExpiry)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if tokens.AccessToken == "" {
		t.Error("GenerateTokenPair() returned empty access token")
	}

	if tokens.RefreshToken == "" {
		t.Error("GenerateTokenPair() returned empty refresh token")
	}

	if tokens.AccessToken == tokens.RefreshToken {
		t.Error("Access and refresh tokens should be different")
	}
}

func TestValidateToken(t *testing.T) {
	userID := "test-user-id"
	email := "test@example.com"
	secret := "test-secret-key"
	accessExpiry := 15 * time.Minute
	refreshExpiry := 7 * 24 * time.Hour

	tokens, err := GenerateTokenPair(userID, email, secret, accessExpiry, refreshExpiry)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Validate access token
	claims, err := ValidateToken(tokens.AccessToken, secret)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("ValidateToken() UserID = %v, want %v", claims.UserID, userID)
	}

	if claims.Email != email {
		t.Errorf("ValidateToken() Email = %v, want %v", claims.Email, email)
	}
}

func TestValidateTokenWithWrongSecret(t *testing.T) {
	userID := "test-user-id"
	email := "test@example.com"
	secret := "test-secret-key"
	wrongSecret := "wrong-secret-key"
	accessExpiry := 15 * time.Minute
	refreshExpiry := 7 * 24 * time.Hour

	tokens, err := GenerateTokenPair(userID, email, secret, accessExpiry, refreshExpiry)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Try to validate with wrong secret
	_, err = ValidateToken(tokens.AccessToken, wrongSecret)
	if err == nil {
		t.Error("ValidateToken() with wrong secret should return error")
	}
}

func TestValidateExpiredToken(t *testing.T) {
	userID := "test-user-id"
	email := "test@example.com"
	secret := "test-secret-key"
	accessExpiry := 1 * time.Millisecond // Very short expiry
	refreshExpiry := 1 * time.Millisecond

	tokens, err := GenerateTokenPair(userID, email, secret, accessExpiry, refreshExpiry)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err = ValidateToken(tokens.AccessToken, secret)
	if err == nil {
		t.Error("ValidateToken() with expired token should return error")
	}
}
