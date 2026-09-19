package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

// userRepo defines repository methods required by UserService.
type userRepo interface {
	GetOrCreate(ctx context.Context, telegramID int64, username string) (*model.User, error)
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetAllUsers(ctx context.Context) ([]model.User, error)
	GetActiveTimezones(ctx context.Context) ([]string, error)
	GetUsersBySlot(ctx context.Context, slot model.SessionSlot, localTime string, timezone string) ([]model.User, error)
	UpdateSlotTime(ctx context.Context, userID int64, slot model.SessionSlot, timeVal *string) error
	UpdateTimezone(ctx context.Context, userID int64, timezone string) error
}

// UserService handles user-related business logic.
type UserService struct {
	userRepo userRepo
}

func NewUserService(userRepo userRepo) *UserService {
	return &UserService{userRepo: userRepo}
}

// GetUser retrieves a user by Telegram ID or creates a new one if not exists.
func (s *UserService) GetUser(ctx context.Context, telegramID int64, username string) (*model.User, error) {
	return s.userRepo.GetOrCreate(ctx, telegramID, username)
}

// GetByID retrieves a user by ID.
func (s *UserService) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

// GetAllUsers returns all registered users.
func (s *UserService) GetAllUsers(ctx context.Context) ([]model.User, error) {
	return s.userRepo.GetAllUsers(ctx)
}

// GetActiveTimezones returns distinct timezones used by registered users.
func (s *UserService) GetActiveTimezones(ctx context.Context) ([]string, error) {
	return s.userRepo.GetActiveTimezones(ctx)
}

// GetUsersBySlot returns users matching slot, local time, and timezone.
func (s *UserService) GetUsersBySlot(
	ctx context.Context,
	slot model.SessionSlot,
	localTime string,
	timezone string,
) ([]model.User, error) {
	return s.userRepo.GetUsersBySlot(ctx, slot, localTime, timezone)
}

// UpdateSlotTime updates the schedule time for a slot. timeVal can be nil to disable the slot.
func (s *UserService) UpdateSlotTime(ctx context.Context, userID int64, slot model.SessionSlot, timeVal *string) error {
	if timeVal != nil {
		if !IsValid30MinTime(*timeVal) {
			return fmt.Errorf("invalid 30-minute time format: %q (expected HH:00 or HH:30)", *timeVal)
		}
	}
	return s.userRepo.UpdateSlotTime(ctx, userID, slot, timeVal)
}

// UpdateTimezone validates and updates the user's timezone.
func (s *UserService) UpdateTimezone(ctx context.Context, userID int64, tz string) error {
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("invalid timezone %q: %w", tz, err)
	}
	return s.userRepo.UpdateTimezone(ctx, userID, tz)
}

// IsValid30MinTime checks if a time string is a valid 24h format on a 30-minute boundary (HH:00 or HH:30).
func IsValid30MinTime(t string) bool {
	if len(t) != 5 || t[2] != ':' {
		return false
	}
	h, errH := strconv.Atoi(t[0:2])
	m, errM := strconv.Atoi(t[3:5])
	if errH != nil || errM != nil {
		return false
	}
	if h < 0 || h > 23 {
		return false
	}
	if m != 0 && m != 30 {
		return false
	}
	return true
}
