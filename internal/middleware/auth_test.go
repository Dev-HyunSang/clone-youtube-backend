package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/auth"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/repository/redis"
	"github.com/gofiber/fiber/v2"
)

type fakeSessionRepo struct {
	sessions map[string]string
}

func (r *fakeSessionRepo) Create(userID, sessionID, refreshTokenID string, ttl time.Duration) error {
	r.sessions[userID+":"+sessionID] = refreshTokenID
	return nil
}

func (r *fakeSessionRepo) Get(userID, sessionID string) (string, error) {
	tokenID, ok := r.sessions[userID+":"+sessionID]
	if !ok {
		return "", redis.ErrSessionNotFound
	}
	return tokenID, nil
}

func (r *fakeSessionRepo) Delete(userID, sessionID string) error {
	delete(r.sessions, userID+":"+sessionID)
	return nil
}

func (r *fakeSessionRepo) DeleteAll(userID string) error {
	for key := range r.sessions {
		delete(r.sessions, key)
	}
	return nil
}

func newTestApp(sessions redis.SessionRepository) *fiber.App {
	app := fiber.New()
	app.Get("/protected", AuthRequired(sessions), func(c *fiber.Ctx) error {
		return c.SendString(c.Locals("userID").(string))
	})
	return app
}

func TestAuthRequired(t *testing.T) {
	sessions := &fakeSessionRepo{sessions: map[string]string{}}
	access, err := auth.GenerateAccessToken("user-123", "session-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sessions.sessions["user-123:session-abc"] = "refresh-token-id"

	app := newTestApp(sessions)

	cases := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"missing header", "", http.StatusUnauthorized},
		{"malformed header", "Token abc", http.StatusUnauthorized},
		{"invalid token", "Bearer not-a-real-token", http.StatusUnauthorized},
		{"valid but revoked session", "Bearer " + mustAccessToken(t, "user-123", "no-such-session"), http.StatusUnauthorized},
		{"valid token and session", "Bearer " + access, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, resp.StatusCode)
			}
		})
	}
}

func mustAccessToken(t *testing.T, userID, sessionID string) string {
	t.Helper()
	token, err := auth.GenerateAccessToken(userID, sessionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return token
}
