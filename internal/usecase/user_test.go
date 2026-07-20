package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/auth"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/domain"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/dto/request"
	"github.com/dev-hyunsang/clone-youtube-backend/internal/repository/redis"
)

type fakeUserRepo struct {
	users map[string]*domain.Users
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[string]*domain.Users{}}
}

func (r *fakeUserRepo) Create(u *domain.Users) error {
	r.users[u.Email] = u
	return nil
}

func (r *fakeUserRepo) FindByEmail(email string) (*domain.Users, error) {
	u, ok := r.users[email]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

type fakeSessionRepo struct {
	sessions map[string]string // "userID:sessionID" -> refresh token id
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: map[string]string{}}
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
		if len(key) > len(userID) && key[:len(userID)+1] == userID+":" {
			delete(r.sessions, key)
		}
	}
	return nil
}

func TestUserUsecase_SignUp(t *testing.T) {
	uc := NewUserUsecase(newFakeUserRepo(), newFakeSessionRepo())

	req := request.UserSignUpRequest{
		Email:     "test@example.com",
		Password:  "password123",
		FirstName: "Hyun",
		LastName:  "Park",
		Nickname:  "hs",
	}

	user, err := uc.SignUp(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Password == req.Password {
		t.Fatal("password was not hashed")
	}

	if _, err := uc.SignUp(req); !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestUserUsecase_SignInRefreshLogout(t *testing.T) {
	sessionRepo := newFakeSessionRepo()
	uc := NewUserUsecase(newFakeUserRepo(), sessionRepo)

	signUpReq := request.UserSignUpRequest{Email: "test@example.com", Password: "password123", FirstName: "Hyun", LastName: "Park", Nickname: "hs"}
	if _, err := uc.SignUp(signUpReq); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	access, refresh, user, err := uc.SignIn(request.UserSignInRequest{Email: "test@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, _, _, err := uc.SignIn(request.UserSignInRequest{Email: "test@example.com", Password: "wrong"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	newAccess, newRefresh, err := uc.RefreshToken(refresh)
	if err != nil {
		t.Fatalf("unexpected error refreshing: %v", err)
	}
	if newAccess == access || newRefresh == refresh {
		t.Fatal("refresh did not rotate tokens")
	}

	// reused (rotated-out) refresh token must be rejected, and must kill the session
	if _, _, err := uc.RefreshToken(refresh); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken for reused refresh token, got %v", err)
	}
	claims, err := auth.ParseToken(newAccess)
	if err != nil {
		t.Fatalf("unexpected error parsing access token: %v", err)
	}
	if _, err := sessionRepo.Get(user.PublicID, claims.SessionID); err == nil {
		t.Fatal("expected session to be revoked after refresh-token reuse")
	}
}

func TestUserUsecase_LogoutAll(t *testing.T) {
	sessionRepo := newFakeSessionRepo()
	uc := NewUserUsecase(newFakeUserRepo(), sessionRepo)

	signUpReq := request.UserSignUpRequest{Email: "multi@example.com", Password: "password123", FirstName: "Hyun", LastName: "Park", Nickname: "hs"}
	if _, err := uc.SignUp(signUpReq); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	signInReq := request.UserSignInRequest{Email: "multi@example.com", Password: "password123"}
	access1, _, user, err := uc.SignIn(signInReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	access2, _, _, err := uc.SignIn(signInReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	claims1, _ := auth.ParseToken(access1)
	claims2, _ := auth.ParseToken(access2)

	if err := uc.LogoutAll(user.PublicID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := sessionRepo.Get(user.PublicID, claims1.SessionID); err == nil {
		t.Fatal("expected device 1 session to be revoked")
	}
	if _, err := sessionRepo.Get(user.PublicID, claims2.SessionID); err == nil {
		t.Fatal("expected device 2 session to be revoked")
	}
}

func TestUserUsecase_Logout(t *testing.T) {
	sessionRepo := newFakeSessionRepo()
	uc := NewUserUsecase(newFakeUserRepo(), sessionRepo)

	signUpReq := request.UserSignUpRequest{Email: "single@example.com", Password: "password123", FirstName: "Hyun", LastName: "Park", Nickname: "hs"}
	if _, err := uc.SignUp(signUpReq); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	access, _, user, err := uc.SignIn(request.UserSignInRequest{Email: "single@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	claims, _ := auth.ParseToken(access)

	if err := uc.Logout(user.PublicID, claims.SessionID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := sessionRepo.Get(user.PublicID, claims.SessionID); err == nil {
		t.Fatal("expected session to be revoked after logout")
	}
}
