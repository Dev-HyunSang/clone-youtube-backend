package usecase

import (
	"errors"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/auth"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/domain"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/dto/request"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/repository/postgressql"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/repository/redis"
	"github.com/google/uuid"
)

var ErrEmailAlreadyExists = errors.New("이미 가입된 이메일입니다")
var ErrInvalidCredentials = errors.New("이메일 또는 비밀번호가 올바르지 않습니다")
var ErrInvalidRefreshToken = errors.New("유효하지 않은 리프레시 토큰입니다")

type UserUsecase interface {
	SignUp(req request.UserSignUpRequest) (*domain.Users, error)
	SignIn(req request.UserSignInRequest) (accessToken, refreshToken string, user *domain.Users, err error)
	RefreshToken(refreshToken string) (accessToken, newRefreshToken string, err error)
	Logout(userID, sessionID string) error
	LogoutAll(userID string) error
}

type userUsecase struct {
	userRepo    postgressql.UserRepository
	sessionRepo redis.SessionRepository
}

func NewUserUsecase(userRepo postgressql.UserRepository, sessionRepo redis.SessionRepository) UserUsecase {
	return &userUsecase{userRepo: userRepo, sessionRepo: sessionRepo}
}

func (u *userUsecase) SignUp(req request.UserSignUpRequest) (*domain.Users, error) {
	if _, err := u.userRepo.FindByEmail(req.Email); err == nil {
		return nil, ErrEmailAlreadyExists
	}

	hashed, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &domain.Users{
		ID:        uuid.NewString(),
		PublicID:  uuid.NewString(),
		Email:     req.Email,
		Password:  hashed,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Nickname:  req.Nickname,
	}

	if err := u.userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (u *userUsecase) SignIn(req request.UserSignInRequest) (string, string, *domain.Users, error) {
	user, err := u.userRepo.FindByEmail(req.Email)
	if err != nil {
		return "", "", nil, ErrInvalidCredentials
	}

	if !auth.CheckPassword(user.Password, req.Password) {
		return "", "", nil, ErrInvalidCredentials
	}

	// a fresh session ID per sign-in identifies this device for logout/logout-all
	accessToken, refreshToken, err := u.issueSession(user.PublicID, uuid.NewString())
	if err != nil {
		return "", "", nil, err
	}

	return accessToken, refreshToken, user, nil
}

func (u *userUsecase) RefreshToken(refreshToken string) (string, string, error) {
	claims, err := auth.ParseToken(refreshToken)
	if err != nil || claims.Type != auth.RefreshTokenType {
		return "", "", ErrInvalidRefreshToken
	}

	currentTokenID, err := u.sessionRepo.Get(claims.Subject, claims.SessionID)
	if err != nil || currentTokenID != claims.ID {
		// mismatch means this refresh token was already rotated out (reuse/theft) - kill the session
		_ = u.sessionRepo.Delete(claims.Subject, claims.SessionID)
		return "", "", ErrInvalidRefreshToken
	}

	return u.issueSession(claims.Subject, claims.SessionID)
}

func (u *userUsecase) Logout(userID, sessionID string) error {
	return u.sessionRepo.Delete(userID, sessionID)
}

func (u *userUsecase) LogoutAll(userID string) error {
	return u.sessionRepo.DeleteAll(userID)
}

func (u *userUsecase) issueSession(userID, sessionID string) (string, string, error) {
	accessToken, err := auth.GenerateAccessToken(userID, sessionID)
	if err != nil {
		return "", "", err
	}
	refreshToken, tokenID, err := auth.GenerateRefreshToken(userID, sessionID)
	if err != nil {
		return "", "", err
	}
	if err := u.sessionRepo.Create(userID, sessionID, tokenID, auth.RefreshTokenTTL); err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}
