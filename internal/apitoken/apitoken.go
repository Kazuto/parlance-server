// Package apitoken creates API tokens and finds who they belong to. A token is "plc_"
// and 32 random bytes in base64url; only its SHA-256 hash is stored.
package apitoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
	"gorm.io/gorm"
)

const (
	// Prefix starts every API token, which tells them from sign-in tokens
	Prefix = "plc_"
	// shownLength is how much of a token its owner sees again, to tell tokens apart
	shownLength = len(Prefix) + 6
	// usedEvery is how often a token's last use is recorded at most
	usedEvery = 5 * time.Minute
)

// ErrInvalid is returned for a token that doesn't exist or expired, or whose user was
// deleted
var ErrInvalid = errors.New("invalid or expired API token")

// Generate returns a new token, the hash to store for it, and its start to show later
func Generate() (secret, hash, shown string, err error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", "", "", err
	}

	secret = Prefix + base64.RawURLEncoding.EncodeToString(random)

	return secret, Hash(secret), secret[:shownLength], nil
}

// Hash returns the hash a token is stored and found by
func Hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))

	return hex.EncodeToString(sum[:])
}

// IsToken tells whether a bearer token is an API token rather than a sign-in token
func IsToken(bearer string) bool {
	return strings.HasPrefix(bearer, Prefix)
}

// Authenticator finds the users of API tokens
type Authenticator struct {
	db  *gorm.DB
	now func() time.Time
}

// NewAuthenticator creates an Authenticator that finds tokens in the database
func NewAuthenticator(db *gorm.DB) *Authenticator {
	return &Authenticator{db: db, now: time.Now}
}

// Authenticate returns the ids of the token and its user, and records that the token
// was used, at most every few minutes
func (a *Authenticator) Authenticate(ctx context.Context, secret string) (tokenID, userID string, err error) {
	db := a.db.WithContext(ctx)

	var tokens []models.APIToken
	if err := db.
		Joins("JOIN users ON users.id = api_tokens.user_id AND users.deleted_at IS NULL").
		Where("api_tokens.token_hash = ?", Hash(secret)).
		Limit(1).
		Find(&tokens).Error; err != nil {
		return "", "", err
	}

	now := a.now()
	if len(tokens) == 0 || tokens[0].ExpiresAt != nil && !now.Before(*tokens[0].ExpiresAt) {
		return "", "", ErrInvalid
	}

	token := tokens[0]
	if token.LastUsedAt == nil || now.Sub(*token.LastUsedAt) >= usedEvery {
		// Requests at the same time record it once.
		if err := db.Model(&models.APIToken{}).
			Where("id = ? AND (last_used_at IS NULL OR last_used_at <= ?)", token.ID, now.Add(-usedEvery)).
			UpdateColumn("last_used_at", now).Error; err != nil {
			return "", "", err
		}
	}

	return token.ID, token.UserID, nil
}
