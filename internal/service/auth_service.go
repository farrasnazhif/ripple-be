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
	Register(ctx context.Context, req request.Register) error
	SendOTP(ctx context.Context, req request.SendOTP) (string, error)
	VerifyOTP(ctx context.Context, req request.VerifyOTP) (*response.UserWithToken, error)
	Login(ctx context.Context, req request.Login) (*response.UserWithToken, error)
}

type authService struct {
	userRepo  repository.UserRepository
	otpRepo   repository.OTPRepository
	mailer    Mailer
	jwtSecret string
	env       string
}

type Mailer interface {
	Send(templateFile, username, email string, data any, isSandbox bool) error
}

func NewAuthService(userRepo repository.UserRepository, otpRepo repository.OTPRepository, mailer Mailer, jwtSecret, env string) AuthService {
	return &authService{
		userRepo:  userRepo,
		otpRepo:   otpRepo,
		mailer:    mailer,
		jwtSecret: jwtSecret,
		env:       env,
	}
}

func (s *authService) Register(ctx context.Context, req request.Register) error {
	_, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil {
		return entity.ErrEmailExists
	}
	if !errors.Is(err, entity.ErrNotFound) {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &entity.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		IsActive:     false,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return err
	}

	otp, err := s.otpRepo.Create(ctx, req.Email, entity.OTPPurposeRegistration, 10*time.Minute)
	if err != nil {
		return err
	}

	vars := struct{ OTPCode string }{OTPCode: otp.Code}
	isSandbox := s.env != "production"
	return s.mailer.Send("user_otp.tmpl", "User", req.Email, vars, isSandbox)
}

func (s *authService) SendOTP(ctx context.Context, req request.SendOTP) (string, error) {
	_, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil {
		return "", entity.ErrEmailExists
	}
	if !errors.Is(err, entity.ErrNotFound) {
		return "", err
	}

	otp, err := s.otpRepo.Create(ctx, req.Email, entity.OTPPurposeRegistration, 10*time.Minute)
	if err != nil {
		return "", err
	}

	vars := struct{ OTPCode string }{OTPCode: otp.Code}
	isSandbox := s.env != "production"
	if err := s.mailer.Send("user_otp.tmpl", "User", req.Email, vars, isSandbox); err != nil {
		return "", err
	}
	return "OTP sent successfully", nil
}

func (s *authService) VerifyOTP(ctx context.Context, req request.VerifyOTP) (*response.UserWithToken, error) {
	_, err := s.otpRepo.Verify(ctx, req.Email, req.Code, entity.OTPPurposeRegistration)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}

	user.IsActive = true
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	token, err := s.generateToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &response.UserWithToken{
		ID: user.ID, Username: user.Username, Email: user.Email,
		IsActive: user.IsActive, Role: user.Role, Credits: user.Credits,
		Token: token,
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
		IsActive: user.IsActive, Role: user.Role, Credits: user.Credits,
		Token: token,
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
