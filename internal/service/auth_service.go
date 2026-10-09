package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	"github.com/farrasnazhif/ripple-be/internal/dto/response"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/repository"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(ctx context.Context, req request.Register) (*response.UserWithToken, error)
	Login(ctx context.Context, req request.Login) (*response.UserWithToken, error)
}

type authService struct {
	userRepo  repository.UserRepository
	jwtSecret string
}

func NewAuthService(userRepo repository.UserRepository, jwtSecret string) AuthService {
	return &authService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

func (s *authService) Register(ctx context.Context, req request.Register) (*response.UserWithToken, error) {
	if len(req.Password) > 72 {
		return nil, entity.ErrPasswordTooLong
	}
	_, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil {
		return nil, entity.ErrEmailExists
	}
	if !errors.Is(err, entity.ErrNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &entity.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		IsActive:     true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	token, err := s.generateToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &response.UserWithToken{
		ID: user.ID, Username: user.Username, Email: user.Email,
		IsActive: user.IsActive,
		Token:    token,
	}, nil
}

func (s *authService) Login(ctx context.Context, req request.Login) (*response.UserWithToken, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, entity.ErrNotFound) {
			return nil, entity.ErrInvalidPassword
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.Password)); err != nil {
		return nil, entity.ErrInvalidPassword
	}

	if !user.IsActive {
		return nil, entity.ErrAccountInactive
	}

	token, err := s.generateToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &response.UserWithToken{
		ID: user.ID, Username: user.Username, Email: user.Email,
		IsActive: user.IsActive,
		Token:    token,
	}, nil
}

func (s *authService) generateToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func ValidateJWTToken(tokenString, secret string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid user_id in token")
	}
	return userID, nil
}
