package usecase

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/domain"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/repository/postgressql"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/service"
	"github.com/google/uuid"
)

var (
	ErrCodeNotFound    = errors.New("발송된 인증 코드가 없습니다")
	ErrCodeExpired     = errors.New("인증 코드가 만료되었습니다")
	ErrInvalidCode     = errors.New("인증 코드가 올바르지 않습니다")
	ErrAlreadyVerified = errors.New("이미 인증된 이메일입니다")
	codeValidFor       = 5 * time.Minute
)

type EmailVerificationUsecase interface {
	SendCode(email string) error
	VerifyCode(email, code string) error
}

type emailVerificationUsecase struct {
	repo   postgressql.EmailVerificationRepository
	sender service.EmailSender
}

func NewEmailVerificationUsecase(repo postgressql.EmailVerificationRepository, sender service.EmailSender) EmailVerificationUsecase {
	return &emailVerificationUsecase{repo: repo, sender: sender}
}

func (u *emailVerificationUsecase) SendCode(email string) error {
	code, err := generateCode()
	if err != nil {
		return err
	}

	v := &domain.EmailVerification{
		ID:        uuid.NewString(),
		Email:     email,
		Code:      code,
		ExpiresAt: time.Now().Add(codeValidFor),
	}
	if err := u.repo.Create(v); err != nil {
		return err
	}

	return u.sender.SendVerificationCode(email, code)
}

func (u *emailVerificationUsecase) VerifyCode(email, code string) error {
	v, err := u.repo.FindLatestByEmail(email)
	if err != nil {
		return ErrCodeNotFound
	}
	if v.Verified {
		return ErrAlreadyVerified
	}
	if time.Now().After(v.ExpiresAt) {
		return ErrCodeExpired
	}
	if v.Code != code {
		return ErrInvalidCode
	}

	return u.repo.MarkVerified(v.ID)
}

func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
