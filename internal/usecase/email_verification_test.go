package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/domain"
)

type fakeEmailVerificationRepo struct {
	byEmail map[string]*domain.EmailVerification
}

func newFakeEmailVerificationRepo() *fakeEmailVerificationRepo {
	return &fakeEmailVerificationRepo{byEmail: map[string]*domain.EmailVerification{}}
}

func (r *fakeEmailVerificationRepo) Create(v *domain.EmailVerification) error {
	r.byEmail[v.Email] = v
	return nil
}

func (r *fakeEmailVerificationRepo) FindLatestByEmail(email string) (*domain.EmailVerification, error) {
	v, ok := r.byEmail[email]
	if !ok {
		return nil, errors.New("not found")
	}
	return v, nil
}

func (r *fakeEmailVerificationRepo) MarkVerified(id string) error {
	for _, v := range r.byEmail {
		if v.ID == id {
			v.Verified = true
		}
	}
	return nil
}

type fakeEmailSender struct {
	sentTo   string
	sentCode string
}

func (s *fakeEmailSender) SendVerificationCode(to, code string) error {
	s.sentTo, s.sentCode = to, code
	return nil
}

func TestEmailVerificationUsecase_SendAndVerify(t *testing.T) {
	repo := newFakeEmailVerificationRepo()
	sender := &fakeEmailSender{}
	uc := NewEmailVerificationUsecase(repo, sender)

	email := "test@example.com"
	if err := uc.SendCode(email); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.sentTo != email || sender.sentCode == "" {
		t.Fatal("verification code was not sent")
	}

	if err := uc.VerifyCode(email, "000000"); !errors.Is(err, ErrInvalidCode) && sender.sentCode != "000000" {
		t.Fatalf("expected ErrInvalidCode for wrong code, got %v", err)
	}

	if err := uc.VerifyCode(email, sender.sentCode); err != nil {
		t.Fatalf("unexpected error verifying correct code: %v", err)
	}

	if err := uc.VerifyCode(email, sender.sentCode); !errors.Is(err, ErrAlreadyVerified) {
		t.Fatalf("expected ErrAlreadyVerified, got %v", err)
	}

	repo.byEmail[email].Verified = false
	repo.byEmail[email].ExpiresAt = time.Now().Add(-time.Minute)
	if err := uc.VerifyCode(email, sender.sentCode); !errors.Is(err, ErrCodeExpired) {
		t.Fatalf("expected ErrCodeExpired, got %v", err)
	}
}
