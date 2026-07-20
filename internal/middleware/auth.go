package middleware

import (
	"net/http"
	"strings"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/auth"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/repository/redis"
	"github.com/gofiber/fiber/v2"
)

func AuthRequired(sessions redis.SessionRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		tokenString, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || tokenString == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "인증 토큰이 필요합니다"})
		}

		claims, err := auth.ParseToken(tokenString)
		if err != nil || claims.Type != auth.AccessTokenType {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "유효하지 않은 토큰입니다"})
		}

		if _, err := sessions.Get(claims.Subject, claims.SessionID); err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "로그아웃된 세션입니다"})
		}

		c.Locals("userID", claims.Subject)
		c.Locals("sessionID", claims.SessionID)
		return c.Next()
	}
}
