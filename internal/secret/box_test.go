package secret

import (
	"errors"
	"strings"
	"testing"
)

func TestBox(t *testing.T) {
	box, err := New("test-secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Run("decrypts what it encrypted", func(t *testing.T) {
		encrypted, err := box.Encrypt("sk-123")
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}

		if strings.Contains(encrypted, "sk-123") || !strings.HasPrefix(encrypted, "v1:") {
			t.Errorf("encrypted: got %q", encrypted)
		}

		plain, err := box.Decrypt(encrypted)
		if err != nil || plain != "sk-123" {
			t.Errorf("Decrypt: got %q, %v", plain, err)
		}
	})

	t.Run("uses a new nonce each time", func(t *testing.T) {
		a, _ := box.Encrypt("sk-123")
		b, _ := box.Encrypt("sk-123")

		if a == b {
			t.Errorf("both encryptions are %q", a)
		}
	})

	t.Run("rejects values from another key or tampered ones", func(t *testing.T) {
		other, _ := New("other-secret")

		encrypted, _ := other.Encrypt("sk-123")
		if _, err := box.Decrypt(encrypted); !errors.Is(err, ErrInvalid) {
			t.Errorf("other key: got %v", err)
		}

		mine, _ := box.Encrypt("sk-123")

		tampered := mine[:len(mine)-2] + "AA"
		for _, value := range []string{tampered, "sk-123", "v1:not-base64!", ""} {
			if _, err := box.Decrypt(value); !errors.Is(err, ErrInvalid) {
				t.Errorf("%q: got %v", value, err)
			}
		}
	})

	t.Run("can't encrypt without a secret", func(t *testing.T) {
		empty, err := New("")
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		if _, err := empty.Encrypt("sk-123"); !errors.Is(err, ErrNoKey) {
			t.Errorf("Encrypt: got %v", err)
		}

		if _, err := empty.Decrypt("v1:abc"); !errors.Is(err, ErrNoKey) {
			t.Errorf("Decrypt: got %v", err)
		}
	})
}
