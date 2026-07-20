package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndParseToken(t *testing.T) {
	access, err := GenerateAccessToken("user-123", "session-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	claims, err := ParseToken(access)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Fatalf("expected user-123, got %s", claims.Subject)
	}
	if claims.Type != AccessTokenType {
		t.Fatalf("expected access token type, got %s", claims.Type)
	}
	if claims.SessionID != "session-abc" {
		t.Fatalf("expected session-abc, got %s", claims.SessionID)
	}

	refresh, tokenID, err := GenerateRefreshToken("user-123", "session-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokenID == "" {
		t.Fatal("expected non-empty refresh token id")
	}
	refreshClaims, err := ParseToken(refresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refreshClaims.Type != RefreshTokenType {
		t.Fatalf("expected refresh token type, got %s", refreshClaims.Type)
	}
	if refreshClaims.ID != tokenID {
		t.Fatalf("expected jti %s, got %s", tokenID, refreshClaims.ID)
	}
}

func TestParseToken_RejectsCrossTypeSecret(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "access-secret")
	t.Setenv("JWT_REFRESH_SECRET", "refresh-secret")

	refresh, _, err := GenerateRefreshToken("user-123", "session-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	claims := &Claims{}
	_, err = jwt.ParseWithClaims(refresh, claims, func(t *jwt.Token) (any, error) {
		return secretFor(AccessTokenType), nil
	})
	if err == nil {
		t.Fatal("expected refresh token signed with refresh secret to fail verification under the access secret")
	}
}

func TestParseToken_RejectsAlgNone(t *testing.T) {
	claims := Claims{
		Type: AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := ParseToken(tokenString); err == nil {
		t.Fatal("expected alg=none token to be rejected")
	}
}
