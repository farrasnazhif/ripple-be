package service

import (
	"context"

	"github.com/farrasnazhif/ripple-be/internal/dto/response"
	"github.com/farrasnazhif/ripple-be/internal/repository"
)

type UserService interface {
	GetProfile(ctx context.Context, userID string) (*response.User, error)
	GetBilling(ctx context.Context, userID string) (*response.UserBilling, error)
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) GetProfile(ctx context.Context, userID string) (*response.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &response.User{
		ID: user.ID, Email: user.Email, Username: user.Username,
		IsActive: user.IsActive, Role: user.Role, Credits: user.Credits,
	}, nil
}

func (s *userService) GetBilling(ctx context.Context, userID string) (*response.UserBilling, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &response.UserBilling{
		UserID: user.ID, Email: user.Email, Username: user.Username,
		Role: user.Role, SubscriptionStatus: user.SubscriptionStatus,
		CreatedAt: user.SubscriptionCreatedAtPtr(), ExpiredAt: user.ExpiredAtPtr(),
	}, nil
}
