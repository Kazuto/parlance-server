package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPasswordErrors(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "empty password",
			password: "",
			wantErr:  false, // bcrypt accepts empty passwords
		},
		{
			name:     "very short password",
			password: "a",
			wantErr:  false,
		},
		{
			name:     "normal password",
			password: "password123",
			wantErr:  false,
		},
		{
			name:     "long password (72 bytes)",
			password: strings.Repeat("a", 72),
			wantErr:  false, // bcrypt max is 72 bytes
		},
		{
			name:        "very long password (over 72 bytes)",
			password:    strings.Repeat("a", 100),
			wantErr:     true, // bcrypt returns error for passwords over 72 bytes
			errContains: "72 bytes",
		},
		{
			name:     "password with special characters",
			password: "p@ssw0rd!#$%^&*()",
			wantErr:  false,
		},
		{
			name:     "password with unicode",
			password: "пароль密码🔒",
			wantErr:  false,
		},
		{
			name:     "password with spaces",
			password: "password with spaces",
			wantErr:  false,
		},
		{
			name:     "password with newlines",
			password: "pass\nword\n123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)

			if tt.wantErr {
				if err == nil {
					t.Error("HashPassword() expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("HashPassword() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("HashPassword() unexpected error = %v", err)
				return
			}

			if hash == "" {
				t.Error("HashPassword() returned empty hash")
			}

			// Verify the hash can be used with CheckPassword
			if err := CheckPassword(tt.password, hash); err != nil {
				t.Errorf("CheckPassword() failed for valid password: %v", err)
			}
		})
	}
}

func TestCheckPasswordErrors(t *testing.T) {
	validPassword := "testpassword123"
	validHash, _ := HashPassword(validPassword)

	tests := []struct {
		name     string
		password string
		hash     string
		wantErr  bool
	}{
		{
			name:     "correct password and hash",
			password: validPassword,
			hash:     validHash,
			wantErr:  false,
		},
		{
			name:     "wrong password",
			password: "wrongpassword",
			hash:     validHash,
			wantErr:  true,
		},
		{
			name:     "empty password",
			password: "",
			hash:     validHash,
			wantErr:  true,
		},
		{
			name:     "empty hash",
			password: validPassword,
			hash:     "",
			wantErr:  true,
		},
		{
			name:     "both empty",
			password: "",
			hash:     "",
			wantErr:  true,
		},
		{
			name:     "invalid hash format",
			password: validPassword,
			hash:     "not-a-valid-bcrypt-hash",
			wantErr:  true,
		},
		{
			name:     "malformed hash",
			password: validPassword,
			hash:     "$2a$10$invalid",
			wantErr:  true,
		},
		{
			name:     "hash from different algorithm",
			password: validPassword,
			hash:     "$1$salt$hash", // MD5 format
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPassword(tt.password, tt.hash)

			if tt.wantErr {
				if err == nil {
					t.Error("CheckPassword() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("CheckPassword() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestPasswordHashingConsistency(t *testing.T) {
	password := "testpassword"

	// Generate multiple hashes
	hash1, err1 := HashPassword(password)
	hash2, err2 := HashPassword(password)
	hash3, err3 := HashPassword(password)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatal("Failed to generate hashes")
	}

	// All hashes should be different (due to salt)
	if hash1 == hash2 || hash1 == hash3 || hash2 == hash3 {
		t.Error("Hashes should be different due to salt")
	}

	// But all should validate the same password
	if err := CheckPassword(password, hash1); err != nil {
		t.Errorf("hash1 should validate password: %v", err)
	}
	if err := CheckPassword(password, hash2); err != nil {
		t.Errorf("hash2 should validate password: %v", err)
	}
	if err := CheckPassword(password, hash3); err != nil {
		t.Errorf("hash3 should validate password: %v", err)
	}
}

func TestPasswordCaseSensitivity(t *testing.T) {
	password := "TestPassword123"
	hash, _ := HashPassword(password)

	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"exact match", "TestPassword123", false},
		{"lowercase", "testpassword123", true},
		{"uppercase", "TESTPASSWORD123", true},
		{"mixed case", "tEstPAssWOrd123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPassword(tt.password, hash)
			if tt.wantErr && err == nil {
				t.Error("CheckPassword() expected error for wrong case")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("CheckPassword() unexpected error = %v", err)
			}
		})
	}
}

func TestBcryptCostValidation(t *testing.T) {
	password := "testpassword"

	// Generate hash with default cost
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	// Extract and verify cost from hash
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost() error = %v", err)
	}

	if cost != bcrypt.DefaultCost {
		t.Errorf("Cost = %v, want %v (bcrypt.DefaultCost)", cost, bcrypt.DefaultCost)
	}
}

func TestCheckPasswordWithDifferentCosts(t *testing.T) {
	password := "testpassword"

	// Generate hashes with different costs
	costs := []int{bcrypt.MinCost, bcrypt.DefaultCost, bcrypt.DefaultCost + 1}

	for _, cost := range costs {
		t.Run(string(rune(cost)), func(t *testing.T) {
			hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
			if err != nil {
				t.Fatalf("Failed to generate hash with cost %d: %v", cost, err)
			}

			// Should be able to check password regardless of cost
			err = CheckPassword(password, string(hash))
			if err != nil {
				t.Errorf("CheckPassword() failed for cost %d: %v", cost, err)
			}
		})
	}
}

func TestPasswordWithNullBytes(t *testing.T) {
	// Test password containing null bytes
	password := "pass\x00word"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	// Should work with exact match including null byte
	err = CheckPassword(password, hash)
	if err != nil {
		t.Errorf("CheckPassword() with null byte error = %v", err)
	}

	// Should fail without null byte
	err = CheckPassword("password", hash)
	if err == nil {
		t.Error("CheckPassword() should fail when null byte is missing")
	}
}
