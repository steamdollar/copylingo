package service

import (
	"context"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockUserRepo struct {
	getOrCreateFn        func(ctx context.Context, telegramID int64, username string) (*model.User, error)
	getByIDFn            func(ctx context.Context, id int64) (*model.User, error)
	getAllUsersFn        func(ctx context.Context) ([]model.User, error)
	getActiveTimezonesFn func(ctx context.Context) ([]string, error)
	getUsersBySlotFn     func(ctx context.Context, slot model.SessionSlot, localTime string, timezone string) ([]model.User, error)
	updateSlotTimeFn     func(ctx context.Context, userID int64, slot model.SessionSlot, timeVal *string) error
	updateTimezoneFn     func(ctx context.Context, userID int64, timezone string) error
}

func (m *mockUserRepo) GetOrCreate(ctx context.Context, tid int64, user string) (*model.User, error) {
	if m.getOrCreateFn != nil {
		return m.getOrCreateFn(ctx, tid, user)
	}
	return nil, nil
}
func (m *mockUserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockUserRepo) GetAllUsers(ctx context.Context) ([]model.User, error) {
	if m.getAllUsersFn != nil {
		return m.getAllUsersFn(ctx)
	}
	return nil, nil
}
func (m *mockUserRepo) GetActiveTimezones(ctx context.Context) ([]string, error) {
	if m.getActiveTimezonesFn != nil {
		return m.getActiveTimezonesFn(ctx)
	}
	return nil, nil
}

func (m *mockUserRepo) GetUsersBySlot(
	ctx context.Context,
	slot model.SessionSlot,
	localTime string,
	timezone string,
) ([]model.User, error) {
	if m.getUsersBySlotFn != nil {
		return m.getUsersBySlotFn(ctx, slot, localTime, timezone)
	}
	return nil, nil
}

func (m *mockUserRepo) UpdateSlotTime(
	ctx context.Context,
	userID int64,
	slot model.SessionSlot,
	timeVal *string,
) error {
	if m.updateSlotTimeFn != nil {
		return m.updateSlotTimeFn(ctx, userID, slot, timeVal)
	}
	return nil
}
func (m *mockUserRepo) UpdateTimezone(ctx context.Context, userID int64, timezone string) error {
	if m.updateTimezoneFn != nil {
		return m.updateTimezoneFn(ctx, userID, timezone)
	}
	return nil
}

func TestGetUser(t *testing.T) {
	ctx := context.Background()
	tid := int64(12345)
	user := "testuser"

	mRepo := &mockUserRepo{
		getOrCreateFn: func(ctx context.Context, telegramID int64, username string) (*model.User, error) {
			if telegramID != tid || username != user {
				t.Errorf("unexpected args: %d, %s", telegramID, username)
			}
			return &model.User{ID: tid, Username: user}, nil
		},
	}

	svc := NewUserService(mRepo)
	u, err := svc.GetUser(ctx, tid, user)

	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if u.ID != tid {
		t.Errorf("expected ID %d, got %d", tid, u.ID)
	}
}

func TestGetAllUsers(t *testing.T) {
	ctx := context.Background()

	mRepo := &mockUserRepo{
		getAllUsersFn: func(ctx context.Context) ([]model.User, error) {
			return []model.User{{ID: 1}, {ID: 2}}, nil
		},
	}

	svc := NewUserService(mRepo)
	users, err := svc.GetAllUsers(ctx)

	if err != nil {
		t.Fatalf("GetAllUsers failed: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestIsValid30MinTime(t *testing.T) {
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

	for _, tc := range tests {
		got := IsValid30MinTime(tc.input)
		if got != tc.want {
			t.Errorf("IsValid30MinTime(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestUpdateSlotTime_ValidatesFormat(t *testing.T) {
	ctx := context.Background()
	var updatedTime *string
	mRepo := &mockUserRepo{
		updateSlotTimeFn: func(ctx context.Context, userID int64, slot model.SessionSlot, timeVal *string) error {
			updatedTime = timeVal
			return nil
		},
	}
	svc := NewUserService(mRepo)

	// Invalid time should return error
	badTime := "08:15"
	if err := svc.UpdateSlotTime(ctx, 1, model.SessionSlotMorningStudy, &badTime); err == nil {
		t.Error("expected error for 08:15, got nil")
	}

	// Valid time should succeed
	validTime := "08:30"
	if err := svc.UpdateSlotTime(ctx, 1, model.SessionSlotMorningStudy, &validTime); err != nil {
		t.Fatalf("unexpected error for 08:30: %v", err)
	}
	if updatedTime == nil || *updatedTime != "08:30" {
		t.Errorf("updatedTime = %v, want 08:30", updatedTime)
	}

	// Nil time (disable slot) should succeed
	if err := svc.UpdateSlotTime(ctx, 1, model.SessionSlotMorningStudy, nil); err != nil {
		t.Fatalf("unexpected error for nil time: %v", err)
	}
	if updatedTime != nil {
		t.Errorf("expected updatedTime to be nil, got %v", *updatedTime)
	}
}

func TestUpdateTimezone_ValidatesIANA(t *testing.T) {
	ctx := context.Background()
	var savedTZ string
	mRepo := &mockUserRepo{
		updateTimezoneFn: func(ctx context.Context, userID int64, timezone string) error {
			savedTZ = timezone
			return nil
		},
	}
	svc := NewUserService(mRepo)

	// Invalid timezone should fail
	if err := svc.UpdateTimezone(ctx, 1, "Invalid/TimeZone_Foo"); err == nil {
		t.Error("expected error for invalid timezone, got nil")
	}

	// Valid timezones should succeed
	for _, tz := range []string{"Asia/Seoul", "America/New_York", "UTC"} {
		if err := svc.UpdateTimezone(ctx, 1, tz); err != nil {
			t.Fatalf("unexpected error for timezone %q: %v", tz, err)
		}
		if savedTZ != tz {
			t.Errorf("savedTZ = %q, want %q", savedTZ, tz)
		}
	}
}
