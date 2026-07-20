package auth

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hashed == "password123" {
		t.Fatal("password was not hashed")
	}

	if !CheckPassword(hashed, "password123") {
		t.Fatal("expected correct password to match")
	}
	if CheckPassword(hashed, "wrong") {
		t.Fatal("expected wrong password to be rejected")
	}
}
