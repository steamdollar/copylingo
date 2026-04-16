package service

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/repository"
)

// UserService handles user-related business logic.
type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

// GetUser retrieves a user by Telegram ID or creates a new one if not exists.
func (s *UserService) GetUser(ctx context.Context, telegramID int64, username string) (*model.User, error) {
	return s.userRepo.GetOrCreate(ctx, telegramID, username)
}

// GetAllUsers returns all registered users.
func (s *UserService) GetAllUsers(ctx context.Context) ([]model.User, error) {
	return s.userRepo.GetAllUsers(ctx)
}
