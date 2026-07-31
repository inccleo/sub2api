package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type dailyCheckinRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewDailyCheckinRepository(client *dbent.Client, sqlDB *sql.DB) service.DailyCheckinRepository {
	return &dailyCheckinRepository{client: client, sql: sqlDB}
}

func (r *dailyCheckinRepository) queryer(ctx context.Context) (sqlQueryer, error) {
	queryer := txAwareSQLExecutor(ctx, r.sql, r.client)
	if queryer == nil {
		return nil, errors.New("sql executor is not configured")
	}
	return queryer, nil
}

func (r *dailyCheckinRepository) GetUserRole(ctx context.Context, userID int64) (string, error) {
	const query = `
		SELECT role
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`
	queryer, err := r.queryer(ctx)
	if err != nil {
		return "", err
	}
	var role string
	err = scanSingleRow(ctx, queryer, query, []any{userID}, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", service.ErrUserNotFound
	}
	return role, err
}

func (r *dailyCheckinRepository) GetByDate(ctx context.Context, userID int64, date time.Time) (*service.DailyCheckinRecord, error) {
	const query = `
		SELECT user_id, checkin_date, base_reward, bonus_reward, total_reward, streak_count, created_at
		FROM daily_checkins
		WHERE user_id = $1 AND checkin_date = $2
	`
	queryer, err := r.queryer(ctx)
	if err != nil {
		return nil, err
	}
	record := &service.DailyCheckinRecord{}
	err = scanSingleRow(ctx, queryer, query, []any{userID, date.Format("2006-01-02")},
		&record.UserID,
		&record.CheckinDate,
		&record.BaseReward,
		&record.BonusReward,
		&record.TotalReward,
		&record.StreakCount,
		&record.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrDailyCheckinNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *dailyCheckinRepository) Create(ctx context.Context, record *service.DailyCheckinRecord) error {
	const query = `
		INSERT INTO daily_checkins (
			user_id, checkin_date, base_reward, bonus_reward, total_reward, streak_count
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, checkin_date) DO NOTHING
		RETURNING id, created_at
	`
	queryer, err := r.queryer(ctx)
	if err != nil {
		return err
	}
	err = scanSingleRow(
		ctx,
		queryer,
		query,
		[]any{
			record.UserID,
			record.CheckinDate.Format("2006-01-02"),
			record.BaseReward,
			record.BonusReward,
			record.TotalReward,
			record.StreakCount,
		},
		&record.ID,
		&record.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrDailyCheckinAlreadyDone
	}
	return err
}

func (r *dailyCheckinRepository) AddUserBalance(ctx context.Context, userID int64, amount float64) (float64, error) {
	const query = `
		UPDATE users
		SET balance = balance + $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING balance
	`
	queryer, err := r.queryer(ctx)
	if err != nil {
		return 0, err
	}
	var balance float64
	err = scanSingleRow(ctx, queryer, query, []any{amount, userID}, &balance)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, service.ErrUserNotFound
	}
	return balance, err
}
