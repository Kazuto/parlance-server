package apitoken_test

import (
	"regexp"
	"testing"

	"github.com/kazuto/parlance-server/internal/apitoken"
)

func TestGenerate(t *testing.T) {
	secret, hash, shown, err := apitoken.Generate()
	if err != nil {
		t.Fatal(err)
	}

	// "plc_" and 32 bytes in base64url without padding
	if !regexp.MustCompile(`^plc_[A-Za-z0-9_-]{43}$`).MatchString(secret) {
		t.Errorf("token = %q", secret)
	}

	if hash != apitoken.Hash(secret) || len(hash) != 64 || hash == apitoken.Hash(secret+"x") {
		t.Errorf("hash = %q", hash)
	}

	if shown != secret[:10] {
		t.Errorf("shown = %q, want the token's first 10 characters", shown)
	}

	if other, _, _, _ := apitoken.Generate(); other == secret {
		t.Error("two tokens are the same")
	}

	if !apitoken.IsToken(secret) || apitoken.IsToken("eyJhbGciOiJIUzI1NiJ9.e30.sig") {
		t.Error("IsToken doesn't tell API tokens from sign-in tokens")
	}
}
