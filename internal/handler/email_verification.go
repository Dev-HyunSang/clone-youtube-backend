package handler

import (
	"errors"
	"net/http"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/dto/request"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/usecase"
	"github.com/gofiber/fiber/v2"
)

type EmailVerificationHandler struct {
	usecase usecase.EmailVerificationUsecase
}

func NewEmailVerificationHandler(usecase usecase.EmailVerificationUsecase) *EmailVerificationHandler {
	return &EmailVerificationHandler{usecase: usecase}
}

func (h *EmailVerificationHandler) SendCode(c *fiber.Ctx) error {
	var req request.SendVerificationCodeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.usecase.SendCode(req.Email); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "인증 코드를 발송했습니다"})
}

func (h *EmailVerificationHandler) VerifyCode(c *fiber.Ctx) error {
	var req request.VerifyEmailCodeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	err := h.usecase.VerifyCode(req.Email, req.Code)
	switch {
	case err == nil:
		return c.Status(http.StatusOK).JSON(fiber.Map{"message": "인증되었습니다"})
	case errors.Is(err, usecase.ErrCodeNotFound), errors.Is(err, usecase.ErrCodeExpired), errors.Is(err, usecase.ErrInvalidCode), errors.Is(err, usecase.ErrAlreadyVerified):
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
}
