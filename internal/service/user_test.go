package service

import (
	"context"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockUserRepository struct {
	getOrCreateFunc func(
		ctx context.Context,
		telegramID int64,
		username string,
	) (*model.User, error)
	getByIDFunc func(
		ctx context.Context,
		id int64,
	) (*model.User, error)
	getAllUsersFunc        func(ctx context.Context) ([]model.User, error)
	getActiveTimezonesFunc func(ctx context.Context) ([]string, error)
	getUsersBySlotFunc     func(
		ctx context.Context,
		slot model.SessionSlot,
		localTime string,
		timezone string,
	) ([]model.User, error)
	updateSlotTimeFunc func(
		ctx context.Context,
		userID int64,
		slot model.SessionSlot,
		timeValue *string,
	) error
	updateTimezoneFunc func(
		ctx context.Context,
		userID int64,
		timezone string,
	) error
}

func (mock *mockUserRepository) GetOrCreate(
	ctx context.Context,
	telegramID int64,
	user string,
) (*model.User, error) {
	if mock.getOrCreateFunc != nil {
		return mock.getOrCreateFunc(
			ctx,
			telegramID,
			user,
		)
	}
	return nil, nil
}
func (mock *mockUserRepository) GetByID(
	ctx context.Context,
	id int64,
) (*model.User, error) {
	if mock.getByIDFunc != nil {
		return mock.getByIDFunc(
			ctx,
			id,
		)
	}
	return nil, nil
}
func (mock *mockUserRepository) GetAllUsers(ctx context.Context) ([]model.User, error) {
	if mock.getAllUsersFunc != nil {
		return mock.getAllUsersFunc(ctx)
	}
	return nil, nil
}
func (mock *mockUserRepository) GetActiveTimezones(ctx context.Context) ([]string, error) {
	if mock.getActiveTimezonesFunc != nil {
		return mock.getActiveTimezonesFunc(ctx)
	}
	return nil, nil
}

func (mock *mockUserRepository) GetUsersBySlot(
	ctx context.Context,
	slot model.SessionSlot,
	localTime string,
	timezone string,
) ([]model.User, error) {
	if mock.getUsersBySlotFunc != nil {
		return mock.getUsersBySlotFunc(
			ctx,
			slot,
			localTime,
			timezone,
		)
	}
	return nil, nil
}

func (mock *mockUserRepository) UpdateSlotTime(
	ctx context.Context,
	userID int64,
	slot model.SessionSlot,
	timeValue *string,
) error {
	if mock.updateSlotTimeFunc != nil {
		return mock.updateSlotTimeFunc(
			ctx,
			userID,
			slot,
			timeValue,
		)
	}
	return nil
}
func (mock *mockUserRepository) UpdateTimezone(
	ctx context.Context,
	userID int64,
	timezone string,
) error {
	if mock.updateTimezoneFunc != nil {
		return mock.updateTimezoneFunc(
			ctx,
			userID,
			timezone,
		)
	}
	return nil
}

func TestGetUser(t *testing.T) {
	ctx := context.Background()
	expectedTelegramID := int64(12345)
	user := "testuser"

	userRepositoryMock := &mockUserRepository{
		getOrCreateFunc: func(
			ctx context.Context,
			telegramID int64,
			username string,
		) (*model.User, error) {
			if telegramID != expectedTelegramID || username != user {
				t.Errorf(
					"unexpected args: %d, %s",
					telegramID,
					username,
				)
			}
			return &model.User{ID: expectedTelegramID, Username: user}, nil
		},
	}

	userService := NewUserService(userRepositoryMock)
	returnedUser, err := userService.GetUser(
		ctx,
		expectedTelegramID,
		user,
	)

	if err != nil {
		t.Fatalf(
			"GetUser failed: %v",
			err,
		)
	}
	if returnedUser.ID != expectedTelegramID {
		t.Errorf(
			"expected ID %d, got %d",
			expectedTelegramID,
			returnedUser.ID,
		)
	}
}

func TestGetAllUsers(t *testing.T) {
	ctx := context.Background()

	userRepositoryMock := &mockUserRepository{
		getAllUsersFunc: func(ctx context.Context) ([]model.User, error) {
			return []model.User{{ID: 1}, {ID: 2}}, nil
		},
	}

	userService := NewUserService(userRepositoryMock)
	users, err := userService.GetAllUsers(ctx)

	if err != nil {
		t.Fatalf(
			"GetAllUsers failed: %v",
			err,
		)
	}
	if len(users) != 2 {
		t.Errorf(
			"expected 2 users, got %d",
			len(users),
		)
	}
}

func TestIsValid30MinuteTime(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"00:00", true},
		{"00:30", true},
		{"08:00", true},
		{"08:30", true},
		{"12:00", true},
		{"16:30", true},
		{"21:00", true},
		{"23:30", true},
		{"08:15", false},
		{"08:45", false},
		{"24:00", false},
		{"-1:00", false},
		{"8:00", false},
		{"invalid", false},
		{"", false},
	}

	for _, testCase := range tests {
		got := IsValid30MinuteTime(testCase.input)
		if got != testCase.want {
			t.Errorf(
				"IsValid30MinuteTime(%q) = %v, want %v",
				testCase.input,
				got,
				testCase.want,
			)
		}
	}
}

func TestUpdateSlotTime_ValidatesFormat(t *testing.T) {
	ctx := context.Background()
	var updatedTime *string
	userRepositoryMock := &mockUserRepository{
		updateSlotTimeFunc: func(
			ctx context.Context,
			userID int64,
			slot model.SessionSlot,
			timeValue *string,
		) error {
			updatedTime = timeValue
			return nil
		},
	}
	userService := NewUserService(userRepositoryMock)

	// Invalid time should return error
	badTime := "08:15"
	if err := userService.UpdateSlotTime(
		ctx,
		1,
		model.SessionSlotMorningStudy,
		&badTime,
	); err == nil {
		t.Error("expected error for 08:15, got nil")
	}

	// Valid time should succeed
	validTime := "08:30"
	if err := userService.UpdateSlotTime(
		ctx,
		1,
		model.SessionSlotMorningStudy,
		&validTime,
	); err != nil {
		t.Fatalf(
			"unexpected error for 08:30: %v",
			err,
		)
	}
	if updatedTime == nil || *updatedTime != "08:30" {
		t.Errorf(
			"updatedTime = %v, want 08:30",
			updatedTime,
		)
	}

	// Nil time (disable slot) should succeed
	if err := userService.UpdateSlotTime(
		ctx,
		1,
		model.SessionSlotMorningStudy,
		nil,
	); err != nil {
		t.Fatalf(
			"unexpected error for nil time: %v",
			err,
		)
	}
	if updatedTime != nil {
		t.Errorf(
			"expected updatedTime to be nil, got %v",
			*updatedTime,
		)
	}
}

func TestUpdateTimezone_ValidatesIANA(t *testing.T) {
	ctx := context.Background()
	var savedTimezone string
	userRepositoryMock := &mockUserRepository{
		updateTimezoneFunc: func(
			ctx context.Context,
			userID int64,
			timezone string,
		) error {
			savedTimezone = timezone
			return nil
		},
	}
	userService := NewUserService(userRepositoryMock)

	// Invalid timezone should fail
	if err := userService.UpdateTimezone(
		ctx,
		1,
		"Invalid/TimeZone_Foo",
	); err == nil {
		t.Error("expected error for invalid timezone, got nil")
	}

	// Valid timezones should succeed
	for _, timezone := range []string{"Asia/Seoul", "America/New_York", "UTC"} {
		if err := userService.UpdateTimezone(
			ctx,
			1,
			timezone,
		); err != nil {
			t.Fatalf(
				"unexpected error for timezone %q: %v",
				timezone,
				err,
			)
		}
		if savedTimezone != timezone {
			t.Errorf(
				"savedTimezone = %q, want %q",
				savedTimezone,
				timezone,
			)
		}
	}
}
