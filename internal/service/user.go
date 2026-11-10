package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

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

type UserService struct {
	userRepository userRepository
}

func NewUserService(repository userRepository) *UserService {
	return &UserService{userRepository: repository}
}

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

func (service *UserService) GetByID(
	ctx context.Context,
	id int64,
) (*model.User, error) {
	return service.userRepository.GetByID(
		ctx,
		id,
	)
}

func (service *UserService) GetAllUsers(ctx context.Context) ([]model.User, error) {
	return service.userRepository.GetAllUsers(ctx)
}

func (service *UserService) GetActiveTimezones(ctx context.Context) ([]string, error) {
	return service.userRepository.GetActiveTimezones(ctx)
}

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

// timeValue can be nil to disable the slot.
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
