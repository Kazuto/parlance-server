package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token types, stored in the "typ" claim so a refresh token cannot be used as an access token
// and vice versa.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type Claims struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// GenerateTokenPair generates both access and refresh tokens
func GenerateTokenPair(userID, email, secret string, accessExpiry, refreshExpiry time.Duration) (*TokenPair, error) {
	accessToken, err := generateToken(userID, email, TokenTypeAccess, secret, accessExpiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := generateToken(userID, email, TokenTypeRefresh, secret, refreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// generateToken creates a JWT token
func generateToken(userID, email, tokenType, secret string, expiry time.Duration) (string, error) {
	claims := &Claims{
		UserID:    userID,
		Email:     email,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secret))
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// ValidateAccessToken validates a JWT token and requires it to be an access token
func ValidateAccessToken(tokenString, secret string) (*Claims, error) {
	return validateTokenOfType(tokenString, secret, TokenTypeAccess)
}

// ValidateRefreshToken validates a JWT token and requires it to be a refresh token
func ValidateRefreshToken(tokenString, secret string) (*Claims, error) {
	return validateTokenOfType(tokenString, secret, TokenTypeRefresh)
}

func validateTokenOfType(tokenString, secret, tokenType string) (*Claims, error) {
	claims, err := ValidateToken(tokenString, secret)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != tokenType {
		return nil, fmt.Errorf("expected %s token", tokenType)
	}

	return claims, nil
}
