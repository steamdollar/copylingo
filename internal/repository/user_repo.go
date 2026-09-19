package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/model"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetOrCreate finds an existing user or creates a new one.
func (r *UserRepository) GetOrCreate(ctx context.Context, telegramID int64, username string) (*model.User, error) {
	user := &model.User{}
	err := r.db.GetContext(ctx, user, `SELECT * FROM users WHERE id = $1`, telegramID)
	if err == nil {
		return user, nil
	}

	// Create new user with default Japanese/N5
	// TODO: ja, N5 defaults should be determined by user input or locale
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, username, language, proficiency_level, streak_days, timezone)
		VALUES ($1, $2, 'ja', 'N5', 0, 'Asia/Seoul')
		ON CONFLICT (id) DO NOTHING
	`, telegramID, username); err != nil {
		return nil, fmt.Errorf("UserRepository.GetOrCreate telegram_id=%d: %w", telegramID, err)
	}

	return r.GetByID(ctx, telegramID)
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	user := &model.User{}
	err := r.db.GetContext(ctx, user, `SELECT * FROM users WHERE id = $1`, id)
	return user, err
}

func (r *UserRepository) Update(ctx context.Context, user *model.User) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET
			username = $2, language = $3, proficiency_level = $4,
			streak_days = $5, streak_last_date = $6,
			morning_session_time = $7, evening_session_time = $8,
			morning_study_time = $9, morning_quiz_time = $10,
			evening_study_time = $11, evening_quiz_time = $12,
			timezone = $13
		WHERE id = $1
	`, user.ID, user.Username, user.Language, user.ProficiencyLevel,
		user.StreakDays, user.StreakLastDate,
		user.MorningSessionTime, user.EveningSessionTime,
		user.MorningStudyTime, user.MorningQuizTime,
		user.EveningStudyTime, user.EveningQuizTime,
		user.Timezone)
	return err
}

// UpdateStreak updates the user's streak count.
func (r *UserRepository) UpdateStreak(ctx context.Context, userID int64) error {
	now := "now()"
	_ = now

	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	today := timeNowDate()
	if user.StreakLastDate != nil && user.StreakLastDate.Format("2006-01-02") == today {
		return nil // Already studied today
	}

	yesterday := timeYesterdayDate()
	newStreak := 1
	if user.StreakLastDate != nil && user.StreakLastDate.Format("2006-01-02") == yesterday {
		newStreak = user.StreakDays + 1
	}

	if _, err := r.db.ExecContext(ctx, `
		UPDATE users SET streak_days = $2, streak_last_date = $3 WHERE id = $1
	`, userID, newStreak, today); err != nil {
		return fmt.Errorf("UserRepository.UpdateStreak user_id=%d: %w", userID, err)
	}
	return nil
}

// GetAllUsers returns all registered users (for scheduled pushes).
func (r *UserRepository) GetAllUsers(ctx context.Context) ([]model.User, error) {
	var users []model.User
	err := r.db.SelectContext(ctx, &users, `SELECT * FROM users ORDER BY id`)
	return users, err
}

func slotColumn(slot model.SessionSlot) (string, error) {
	switch slot {
	case model.SessionSlotMorningStudy:
		return "morning_study_time", nil
	case model.SessionSlotMorningQuiz:
		return "morning_quiz_time", nil
	case model.SessionSlotEveningStudy:
		return "evening_study_time", nil
	case model.SessionSlotEveningQuiz:
		return "evening_quiz_time", nil
	default:
		return "", fmt.Errorf("unknown session slot: %s", slot)
	}
}

// GetActiveTimezones returns all distinct, non-empty timezones configured by users.
func (r *UserRepository) GetActiveTimezones(ctx context.Context) ([]string, error) {
	var timezones []string
	query := `SELECT DISTINCT timezone FROM users WHERE timezone IS NOT NULL AND timezone != '' ORDER BY timezone`
	if err := r.db.SelectContext(ctx, &timezones, query); err != nil {
		return nil, fmt.Errorf("UserRepository.GetActiveTimezones: %w", err)
	}
	return timezones, nil
}

// GetUsersBySlot retrieves users scheduled for a specific slot at localTime in timezone.
func (r *UserRepository) GetUsersBySlot(
	ctx context.Context,
	slot model.SessionSlot,
	localTime string,
	timezone string,
) ([]model.User, error) {
	col, err := slotColumn(slot)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT * FROM users
		WHERE timezone = $1 AND %s = $2
		ORDER BY id
	`, col)

	var users []model.User
	if err := r.db.SelectContext(ctx, &users, query, timezone, localTime); err != nil {
		return nil, fmt.Errorf(
			"UserRepository.GetUsersBySlot slot=%s tz=%s time=%s: %w",
			slot,
			timezone,
			localTime,
			err,
		)
	}
	return users, nil
}

// UpdateSlotTime updates a specific slot time for a user (nil to disable).
func (r *UserRepository) UpdateSlotTime(
	ctx context.Context,
	userID int64,
	slot model.SessionSlot,
	timeVal *string,
) error {
	col, err := slotColumn(slot)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`UPDATE users SET %s = $2 WHERE id = $1`, col)
	result, err := r.db.ExecContext(ctx, query, userID, timeVal)
	if err != nil {
		return fmt.Errorf("UserRepository.UpdateSlotTime user_id=%d slot=%s: %w", userID, slot, err)
	}
	if rows, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("UserRepository.UpdateSlotTime user_id=%d rows: %w", userID, err)
	} else if rows == 0 {
		return fmt.Errorf("UserRepository.UpdateSlotTime user_id=%d: user not found", userID)
	}
	return nil
}

// UpdateTimezone updates the user's timezone.
func (r *UserRepository) UpdateTimezone(ctx context.Context, userID int64, timezone string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE users SET timezone = $2 WHERE id = $1`, userID, timezone)
	if err != nil {
		return fmt.Errorf("UserRepository.UpdateTimezone user_id=%d: %w", userID, err)
	}
	if rows, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("UserRepository.UpdateTimezone user_id=%d rows: %w", userID, err)
	} else if rows == 0 {
		return fmt.Errorf("UserRepository.UpdateTimezone user_id=%d: user not found", userID)
	}
	return nil
}

func timeNowDate() string {
	return timeNow().Format("2006-01-02")
}

func timeYesterdayDate() string {
	return timeNow().AddDate(0, 0, -1).Format("2006-01-02")
}
