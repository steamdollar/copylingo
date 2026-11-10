package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

// userRepository defines repository methods required by UserService.
type userRepository interface {
	GetOrCreate(
		ctx context.Context,
		telegramID int64,
		username string,
	) (*model.User, error)
	GetByID(
		ctx context.Context,
		id int64,
	) (*model.User, error)
	GetAllUsers(ctx context.Context) ([]model.User, error)
	GetActiveTimezones(ctx context.Context) ([]string, error)
	GetUsersBySlot(
		ctx context.Context,
		slot model.SessionSlot,
		localTime string,
		timezone string,
	) ([]model.User, error)
	UpdateSlotTime(
		ctx context.Context,
		userID int64,
		slot model.SessionSlot,
		timeValue *string,
	) error
	UpdateTimezone(
		ctx context.Context,
		userID int64,
		timezone string,
	) error
}

// UserService handles user-related business logic.
type UserService struct {
	userRepository userRepository
}

func NewUserService(repository userRepository) *UserService {
	return &UserService{userRepository: repository}
}

// GetUser retrieves a user by Telegram ID or creates a new one if not exists.
func (service *UserService) GetUser(
	ctx context.Context,
	telegramID int64,
	username string,
) (*model.User, error) {
	return service.userRepository.GetOrCreate(
		ctx,
		telegramID,
		username,
	)
}

// GetByID retrieves a user by ID.
func (service *UserService) GetByID(
	ctx context.Context,
	id int64,
) (*model.User, error) {
	return service.userRepository.GetByID(
		ctx,
		id,
	)
}

// GetAllUsers returns all registered users.
func (service *UserService) GetAllUsers(ctx context.Context) ([]model.User, error) {
	return service.userRepository.GetAllUsers(ctx)
}

// GetActiveTimezones returns distinct timezones used by registered users.
func (service *UserService) GetActiveTimezones(ctx context.Context) ([]string, error) {
	return service.userRepository.GetActiveTimezones(ctx)
}

// GetUsersBySlot returns users matching slot, local time, and timezone.
func (service *UserService) GetUsersBySlot(
	ctx context.Context,
	slot model.SessionSlot,
	localTime string,
	timezone string,
) ([]model.User, error) {
	return service.userRepository.GetUsersBySlot(
		ctx,
		slot,
		localTime,
		timezone,
	)
}

// UpdateSlotTime updates the schedule time for a slot. timeValue can be nil to disable the slot.
func (service *UserService) UpdateSlotTime(
	ctx context.Context,
	userID int64,
	slot model.SessionSlot,
	timeValue *string,
) error {
	if timeValue != nil {
		if !IsValid30MinuteTime(*timeValue) {
			return fmt.Errorf(
				"invalid 30-minute time format: %q (expected HH:00 or HH:30)",
				*timeValue,
			)
		}
	}
	return service.userRepository.UpdateSlotTime(
		ctx,
		userID,
		slot,
		timeValue,
	)
}

// UpdateTimezone validates and updates the user's timezone.
func (service *UserService) UpdateTimezone(
	ctx context.Context,
	userID int64,
	timezone string,
) error {
	if _, err := time.LoadLocation(timezone); err != nil {
		return fmt.Errorf(
			"invalid timezone %q: %w",
			timezone,
			err,
		)
	}
	return service.userRepository.UpdateTimezone(
		ctx,
		userID,
		timezone,
	)
}

// IsValid30MinuteTime checks if a time string is a valid 24h format on a 30-minute boundary (HH:00 or HH:30).
func IsValid30MinuteTime(timeText string) bool {
	if len(timeText) != 5 || timeText[2] != ':' {
		return false
	}
	hour, hourErr := strconv.Atoi(timeText[0:2])
	minute, minuteErr := strconv.Atoi(timeText[3:5])
	if hourErr != nil || minuteErr != nil {
		return false
	}
	if hour < 0 || hour > 23 {
		return false
	}
	if minute != 0 && minute != 30 {
		return false
	}
	return true
}
