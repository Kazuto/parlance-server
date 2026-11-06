// Package secret encrypts values the server stores but must not keep readable,
// such as API keys of translation providers.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

// prefix marks the format, so a later change of algorithm can tell values apart.
const prefix = "v1:"

// ErrNoKey means no encryption key is configured.
var ErrNoKey = errors.New("no encryption key is configured")

// ErrInvalid means a value can't be decrypted, e.g. after the key changed.
var ErrInvalid = errors.New("the value can't be decrypted")

// Box encrypts with AES-256-GCM, using a key derived from a configured secret.
type Box struct {
	aead cipher.AEAD
}

// New derives the key from secret. Without a secret the box can't encrypt.
func New(secret string) (*Box, error) {
	if secret == "" {
		return &Box{}, nil
	}

	key, err := hkdf.Key(sha256.New, []byte(secret), nil, "parlance secret box v1", 32)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &Box{aead: aead}, nil
}

// Encrypt returns the value encrypted, with a fresh nonce each time.
func (b *Box) Encrypt(plain string) (string, error) {
	if b == nil || b.aead == nil {
		return "", ErrNoKey
	}

	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	sealed := b.aead.Seal(nonce, nonce, []byte(plain), nil)

	return prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt returns the value Encrypt was given.
func (b *Box) Decrypt(encrypted string) (string, error) {
	if b == nil || b.aead == nil {
		return "", ErrNoKey
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encrypted, prefix))
	if !strings.HasPrefix(encrypted, prefix) || err != nil || len(data) < b.aead.NonceSize() {
		return "", ErrInvalid
	}

	nonce, sealed := data[:b.aead.NonceSize()], data[b.aead.NonceSize():]

	plain, err := b.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", ErrInvalid
	}

	return string(plain), nil
}
