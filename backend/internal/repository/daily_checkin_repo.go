package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

func (r *dailyCheckinRepository) executor(ctx context.Context) (sqlQueryExecutor, error) {
	exec := txAwareSQLExecutor(ctx, r.sql, r.client)
	if exec == nil {
		return nil, errors.New("sql executor is not configured")
	}
	return exec, nil
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

func (r *dailyCheckinRepository) LockUserForCheckin(ctx context.Context, userID int64) (err error) {
	queryer, err := r.queryer(ctx)
	if err != nil {
		return err
	}
	rows, err := queryer.QueryContext(
		ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended('daily_checkin:' || $1::text, 0))`,
		userID,
	)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return rows.Err()
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

func (r *dailyCheckinRepository) ListByUser(ctx context.Context, userID int64, limit int) ([]service.DailyCheckinRecord, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	const query = `
		SELECT id, user_id, checkin_date, base_reward, bonus_reward, total_reward, streak_count, created_at
		FROM daily_checkins
		WHERE user_id = $1
		ORDER BY checkin_date DESC, id DESC
		LIMIT $2
	`
	queryer, err := r.queryer(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queryer.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	records := make([]service.DailyCheckinRecord, 0)
	for rows.Next() {
		var record service.DailyCheckinRecord
		if err := rows.Scan(
			&record.ID,
			&record.UserID,
			&record.CheckinDate,
			&record.BaseReward,
			&record.BonusReward,
			&record.TotalReward,
			&record.StreakCount,
			&record.CreatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (r *dailyCheckinRepository) List(ctx context.Context, filter service.DailyCheckinListFilter) ([]service.DailyCheckinListItem, int64, error) {
	exec, err := r.executor(ctx)
	if err != nil {
		return nil, 0, err
	}

	where, args := buildDailyCheckinListWhere(filter)

	countQuery := `
		SELECT COUNT(*)
		FROM daily_checkins dc
		JOIN users u ON u.id = dc.user_id
	` + where
	var total int64
	if err := scanSingleRow(ctx, exec, countQuery, args, &total); err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	limitPlaceholder := "$" + strconv.Itoa(len(args)-1)
	offsetPlaceholder := "$" + strconv.Itoa(len(args))

	listQuery := `
		SELECT dc.id,
		       dc.user_id,
		       COALESCE(u.email, ''),
		       COALESCE(u.username, ''),
		       dc.checkin_date,
		       dc.base_reward,
		       dc.bonus_reward,
		       dc.total_reward,
		       dc.streak_count,
		       dc.created_at
		FROM daily_checkins dc
		JOIN users u ON u.id = dc.user_id
	` + where + `
		ORDER BY dc.checkin_date DESC, dc.id DESC
		LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder

	rows, err := exec.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.DailyCheckinListItem, 0)
	for rows.Next() {
		var item service.DailyCheckinListItem
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.UserEmail,
			&item.Username,
			&item.CheckinDate,
			&item.BaseReward,
			&item.BonusReward,
			&item.TotalReward,
			&item.StreakCount,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *dailyCheckinRepository) AdminStats(ctx context.Context, today, yesterday, last7Start time.Time) (*service.DailyCheckinAdminStats, error) {
	exec, err := r.executor(ctx)
	if err != nil {
		return nil, err
	}

	const query = `
		SELECT
			COUNT(*) FILTER (WHERE checkin_date = $1::date)::bigint AS today_checkins,
			COUNT(*) FILTER (WHERE checkin_date = $2::date)::bigint AS yesterday_checkins,
			COUNT(*) FILTER (WHERE checkin_date >= $3::date)::bigint AS last_7_days_checkins,
			COUNT(*)::bigint AS total_checkins,
			COUNT(DISTINCT user_id)::bigint AS unique_users,
			COALESCE(SUM(total_reward) FILTER (WHERE checkin_date = $1::date), 0)::float8 AS today_reward_total,
			COALESCE(SUM(total_reward) FILTER (WHERE checkin_date = $2::date), 0)::float8 AS yesterday_reward_total,
			COALESCE(SUM(total_reward) FILTER (WHERE checkin_date >= $3::date), 0)::float8 AS last_7_days_reward_total,
			COALESCE(SUM(total_reward), 0)::float8 AS total_reward,
			COALESCE(SUM(bonus_reward), 0)::float8 AS total_bonus_reward,
			COUNT(*) FILTER (WHERE checkin_date = $1::date AND bonus_reward > 0)::bigint AS today_bonus_count
		FROM daily_checkins
	`
	stats := &service.DailyCheckinAdminStats{}
	err = scanSingleRow(
		ctx,
		exec,
		query,
		[]any{
			today.Format("2006-01-02"),
			yesterday.Format("2006-01-02"),
			last7Start.Format("2006-01-02"),
		},
		&stats.TodayCheckins,
		&stats.YesterdayCheckins,
		&stats.Last7DaysCheckins,
		&stats.TotalCheckins,
		&stats.UniqueUsers,
		&stats.TodayRewardTotal,
		&stats.YesterdayRewardTotal,
		&stats.Last7DaysRewardTotal,
		&stats.TotalReward,
		&stats.TotalBonusReward,
		&stats.TodayBonusCount,
	)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func buildDailyCheckinListWhere(filter service.DailyCheckinListFilter) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)

	if filter.UserID != nil && *filter.UserID > 0 {
		args = append(args, *filter.UserID)
		clauses = append(clauses, fmt.Sprintf("dc.user_id = $%d", len(args)))
	}

	search := strings.TrimSpace(filter.Search)
	if search != "" {
		if len(search) > 100 {
			search = search[:100]
		}
		args = append(args, "%"+search+"%")
		placeholder := fmt.Sprintf("$%d", len(args))
		clauses = append(clauses, fmt.Sprintf(
			"(u.email ILIKE %s OR u.username ILIKE %s OR CAST(dc.user_id AS TEXT) ILIKE %s)",
			placeholder, placeholder, placeholder,
		))
	}

	if filter.StartDate != nil {
		args = append(args, filter.StartDate.Format("2006-01-02"))
		clauses = append(clauses, fmt.Sprintf("dc.checkin_date >= $%d::date", len(args)))
	}
	if filter.EndDate != nil {
		args = append(args, filter.EndDate.Format("2006-01-02"))
		clauses = append(clauses, fmt.Sprintf("dc.checkin_date <= $%d::date", len(args)))
	}

	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}
