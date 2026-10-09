package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"golang.org/x/crypto/bcrypt"
)

type authUserRepo struct{ user *entity.User }

func (r *authUserRepo) Create(_ context.Context, user *entity.User) error {
	if r.user != nil {
		return entity.ErrDuplicateEmail
	}
	user.ID = "new-user-id"
	r.user = user
	return nil
}

func (r *authUserRepo) GetByID(_ context.Context, id string) (*entity.User, error) {
	if r.user != nil && r.user.ID == id {
		return r.user, nil
	}
	return nil, entity.ErrNotFound
}

func (r *authUserRepo) GetByEmail(_ context.Context, email string) (*entity.User, error) {
	if r.user != nil && r.user.Email == email {
		return r.user, nil
	}
	return nil, entity.ErrNotFound
}

func TestRegisterActivatesAccountAndReturnsUsableToken(t *testing.T) {
	repo := &authUserRepo{}
	svc := NewAuthService(repo, "test-secret")
	registration := request.Register{Username: "studio", Email: "studio@example.com", Password: "a long test password"}

	result, err := svc.Register(context.Background(), registration)
	if err != nil {
		t.Fatal(err)
	}
	if !repo.user.IsActive || !result.IsActive || result.Token == "" {
		t.Fatalf("registration did not activate account and return token: %+v", result)
	}
	if err := bcrypt.CompareHashAndPassword(repo.user.PasswordHash, []byte(registration.Password)); err != nil {
		t.Fatalf("password was not hashed correctly: %v", err)
	}
	if userID, err := ValidateJWTToken(result.Token, "test-secret"); err != nil || userID != repo.user.ID {
		t.Fatalf("registration token is unusable: id=%q err=%v", userID, err)
	}
	if _, err := svc.Login(context.Background(), request.Login{Email: registration.Email, Password: registration.Password}); err != nil {
		t.Fatalf("new account cannot log in: %v", err)
	}
	if _, err := svc.Register(context.Background(), registration); !errors.Is(err, entity.ErrEmailExists) {
		t.Fatalf("duplicate registration returned %v", err)
	}
}

func TestRegisterRejectsPasswordBeyondBcryptLimit(t *testing.T) {
	svc := NewAuthService(&authUserRepo{}, "test-secret")
	_, err := svc.Register(context.Background(), request.Register{Username: "studio", Email: "studio@example.com", Password: strings.Repeat("é", 37)})
	if !errors.Is(err, entity.ErrPasswordTooLong) {
		t.Fatalf("expected byte-length validation, got %v", err)
	}
}
