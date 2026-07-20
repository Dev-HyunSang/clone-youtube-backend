package handler

import (
	"errors"
	"net/http"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/dto/request"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/dto/response"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/usecase"
	"github.com/gofiber/fiber/v2"
)

type UserHandler struct {
	userUsecase usecase.UserUsecase
}

func NewUserHandler(userUsecase usecase.UserUsecase) *UserHandler {
	return &UserHandler{userUsecase: userUsecase}
}

func (h *UserHandler) SignUp(c *fiber.Ctx) error {
	var req request.UserSignUpRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	user, err := h.userUsecase.SignUp(req)
	if err != nil {
		if errors.Is(err, usecase.ErrEmailAlreadyExists) {
			return c.Status(http.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusCreated).JSON(response.UserResponse{
		ID:        user.PublicID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Nickname:  user.Nickname,
		CreatedAt: user.CreatedAt,
	})
}

func (h *UserHandler) SignIn(c *fiber.Ctx) error {
	var req request.UserSignInRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	accessToken, refreshToken, user, err := h.userUsecase.SignIn(req)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidCredentials) {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusOK).JSON(response.SignInResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: response.UserResponse{
			ID:        user.PublicID,
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Nickname:  user.Nickname,
			CreatedAt: user.CreatedAt,
		},
	})
}

func (h *UserHandler) RefreshToken(c *fiber.Ctx) error {
	var req request.RefreshTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	accessToken, refreshToken, err := h.userUsecase.RefreshToken(req.RefreshToken)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidRefreshToken) {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusOK).JSON(response.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

func (h *UserHandler) Logout(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)
	sessionID, _ := c.Locals("sessionID").(string)

	if err := h.userUsecase.Logout(userID, sessionID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "로그아웃되었습니다"})
}

func (h *UserHandler) LogoutAll(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(string)

	if err := h.userUsecase.LogoutAll(userID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "모든 기기에서 로그아웃되었습니다"})
}
