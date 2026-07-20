package auth

import (
	"errors"
	"time"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("유효하지 않은 토큰입니다")

const (
	AccessTokenType  = "access"
	RefreshTokenType = "refresh"

	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
)

type Claims struct {
	Type      string `json:"typ"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(userID, sessionID string) (string, error) {
	token, _, err := generateToken(userID, sessionID, AccessTokenType, AccessTokenTTL)
	return token, err
}

// GenerateRefreshToken also returns the token's jti so callers can record it
// as the session's currently-valid refresh token (for rotation/reuse checks).
func GenerateRefreshToken(userID, sessionID string) (token string, tokenID string, err error) {
	return generateToken(userID, sessionID, RefreshTokenType, RefreshTokenTTL)
}

func secretFor(tokenType string) []byte {
	if tokenType == RefreshTokenType {
		return []byte(config.GetEnv("JWT_REFRESH_SECRET"))
	}
	return []byte(config.GetEnv("JWT_ACCESS_SECRET"))
}

func generateToken(userID, sessionID, tokenType string, ttl time.Duration) (string, string, error) {
	tokenID := uuid.NewString()
	claims := Claims{
		Type:      tokenType,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secretFor(tokenType))
	return signed, tokenID, err
}

// ParseToken verifies the token against both secrets since the caller
// doesn't know the token type until the claims are decoded; the type-specific
// checks at the call sites (middleware, refresh usecase) reject any token
// whose declared Type doesn't match the secret it was actually signed with.
func ParseToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		unverified := t.Claims.(*Claims)
		return secretFor(unverified.Type), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
