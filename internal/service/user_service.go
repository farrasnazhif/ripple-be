package service

import (
	"context"

	"github.com/farrasnazhif/ripple-be/internal/dto/response"
	"github.com/farrasnazhif/ripple-be/internal/repository"
)

type UserService interface {
	GetProfile(ctx context.Context, userID string) (*response.User, error)
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
		IsActive: user.IsActive,
	}, nil
}
