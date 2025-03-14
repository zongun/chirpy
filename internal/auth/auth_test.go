package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	pass := "secret-pass"
	hash, err := HashPassword(pass)

	if err != nil {
		t.Errorf("Wanted no error, got %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)); err != nil {
		t.Errorf("Wanted no error, got %v", err)
	}
}

func TestVerifyPassword(t *testing.T) {
	pass := "secret-pass"
	hash, _ := HashPassword(pass)

	if err := VerifyPassword(pass, hash); err != nil {
		t.Errorf("Wanted no error, got %v", err)
	}
}
