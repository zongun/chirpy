package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestCreateAndValidateJWT(t *testing.T) {
	var (
		secret  = "secret"
		expires = time.Second * 5
	)
	userID, _ := uuid.NewUUID()

	tokenString, err := CreateJWT(userID, secret, expires)
	if err != nil {
		t.Errorf("Expected no error, got: %q", err)
		return
	}

	validUserID, err := ValidateJWT(tokenString, "secret")
	if err != nil {
		t.Errorf("Expexted no error, got: %q", err)
		return
	}

	if userID != validUserID {
		t.Errorf("Expected %v to be equal to %v", userID, validUserID)
		return
	}
}
