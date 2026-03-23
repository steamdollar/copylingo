package repository

import (
	"context"

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
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO users (id, username, language, proficiency_level, streak_days, timezone)
		VALUES ($1, $2, 'ja', 'N5', 0, 'Asia/Seoul')
		ON CONFLICT (id) DO NOTHING
	`, telegramID, username)
	if err != nil {
		return nil, err
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
			timezone = $9
		WHERE id = $1
	`, user.ID, user.Username, user.Language, user.ProficiencyLevel,
		user.StreakDays, user.StreakLastDate,
		user.MorningSessionTime, user.EveningSessionTime,
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

	_, err = r.db.ExecContext(ctx, `
		UPDATE users SET streak_days = $2, streak_last_date = $3 WHERE id = $1
	`, userID, newStreak, today)
	return err
}

// GetAllUsers returns all registered users (for scheduled pushes).
func (r *UserRepository) GetAllUsers(ctx context.Context) ([]model.User, error) {
	var users []model.User
	err := r.db.SelectContext(ctx, &users, `SELECT * FROM users ORDER BY id`)
	return users, err
}

func timeNowDate() string {
	return timeNow().Format("2006-01-02")
}

func timeYesterdayDate() string {
	return timeNow().AddDate(0, 0, -1).Format("2006-01-02")
}
