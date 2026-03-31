package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if hash == "" {
		t.Error("HashPassword() returned empty hash")
	}

	if hash == password {
		t.Error("HashPassword() returned unhashed password")
	}
}

func TestCheckPassword(t *testing.T) {
	password := "testpassword123"
	wrongPassword := "wrongpassword"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	// Test correct password
	err = CheckPassword(password, hash)
	if err != nil {
		t.Errorf("CheckPassword() with correct password error = %v", err)
	}

	// Test wrong password
	err = CheckPassword(wrongPassword, hash)
	if err == nil {
		t.Error("CheckPassword() with wrong password should return error")
	}
}

func TestHashPasswordDifferentHashes(t *testing.T) {
	password := "testpassword123"

	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	// Bcrypt should generate different hashes due to salt
	if hash1 == hash2 {
		t.Error("HashPassword() should generate different hashes for same password")
	}

	// But both should validate
	if err := CheckPassword(password, hash1); err != nil {
		t.Errorf("CheckPassword() failed for hash1: %v", err)
	}

	if err := CheckPassword(password, hash2); err != nil {
		t.Errorf("CheckPassword() failed for hash2: %v", err)
	}
}
