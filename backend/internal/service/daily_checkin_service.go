package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	DailyCheckinRewardDefault      = 0.01
	DailyCheckinWeeklyBonusDefault = 0.05
	DailyCheckinRewardLimit        = 10000.0
	dailyCheckinBonusInterval      = 7
	dailyCheckinCacheTimeout       = 5 * time.Second
)

var (
	ErrDailyCheckinNotFound    = infraerrors.NotFound("DAILY_CHECKIN_NOT_FOUND", "daily check-in record not found")
	ErrDailyCheckinDisabled    = infraerrors.Forbidden("DAILY_CHECKIN_DISABLED", "daily check-in is disabled")
	ErrDailyCheckinRole        = infraerrors.Forbidden("DAILY_CHECKIN_ROLE_FORBIDDEN", "only regular users can use daily check-in")
	ErrDailyCheckinAlreadyDone = infraerrors.Conflict("DAILY_CHECKIN_ALREADY_DONE", "already checked in today")
)

type DailyCheckinRecord struct {
	ID          int64
	UserID      int64
	CheckinDate time.Time
	BaseReward  float64
	BonusReward float64
	TotalReward float64
	StreakCount int
	CreatedAt   time.Time
}

// DailyCheckinListItem is an admin-facing check-in record with user identity fields.
type DailyCheckinListItem struct {
	DailyCheckinRecord
	UserEmail string
	Username  string
}

// DailyCheckinListFilter controls admin check-in history pagination and search.
type DailyCheckinListFilter struct {
	Page      int
	PageSize  int
	Search    string
	UserID    *int64
	StartDate *time.Time
	EndDate   *time.Time
}

type DailyCheckinRepository interface {
	GetUserRole(ctx context.Context, userID int64) (string, error)
	GetByDate(ctx context.Context, userID int64, date time.Time) (*DailyCheckinRecord, error)
	LockUserForCheckin(ctx context.Context, userID int64) error
	Create(ctx context.Context, record *DailyCheckinRecord) error
	AddUserBalance(ctx context.Context, userID int64, amount float64) (float64, error)
	ListByUser(ctx context.Context, userID int64, limit int) ([]DailyCheckinRecord, error)
	List(ctx context.Context, filter DailyCheckinListFilter) ([]DailyCheckinListItem, int64, error)
	AdminStats(ctx context.Context, today, yesterday, last7Start time.Time) (*DailyCheckinAdminStats, error)
}

type DailyCheckinStatus struct {
	Enabled        bool     `json:"enabled"`
	CheckedInToday bool     `json:"checked_in_today"`
	DailyReward    float64  `json:"daily_reward"`
	WeeklyBonus    float64  `json:"weekly_bonus"`
	CurrentStreak  int      `json:"current_streak"`
	DaysUntilBonus int      `json:"days_until_bonus"`
	TodayReward    *float64 `json:"today_reward,omitempty"`
	ServerTimezone string   `json:"server_timezone"`
}

type DailyCheckinResult struct {
	RewardAmount  float64 `json:"reward_amount"`
	BaseReward    float64 `json:"base_reward"`
	BonusReward   float64 `json:"bonus_reward"`
	NewBalance    float64 `json:"new_balance"`
	CurrentStreak int     `json:"current_streak"`
	CheckedInAt   string  `json:"checked_in_at"`
}

// DailyCheckinHistoryItem is a user-facing check-in history row.
type DailyCheckinHistoryItem struct {
	ID          int64   `json:"id"`
	CheckinDate string  `json:"checkin_date"`
	BaseReward  float64 `json:"base_reward"`
	BonusReward float64 `json:"bonus_reward"`
	TotalReward float64 `json:"total_reward"`
	StreakCount int     `json:"streak_count"`
	CreatedAt   string  `json:"created_at"`
}

// DailyCheckinAdminItem is an admin-facing check-in history row.
type DailyCheckinAdminItem struct {
	ID          int64   `json:"id"`
	UserID      int64   `json:"user_id"`
	UserEmail   string  `json:"user_email"`
	Username    string  `json:"username"`
	CheckinDate string  `json:"checkin_date"`
	BaseReward  float64 `json:"base_reward"`
	BonusReward float64 `json:"bonus_reward"`
	TotalReward float64 `json:"total_reward"`
	StreakCount int     `json:"streak_count"`
	CreatedAt   string  `json:"created_at"`
}

// DailyCheckinAdminStats is the admin dashboard summary for check-ins.
type DailyCheckinAdminStats struct {
	TodayCheckins        int64   `json:"today_checkins"`
	YesterdayCheckins    int64   `json:"yesterday_checkins"`
	Last7DaysCheckins    int64   `json:"last_7_days_checkins"`
	TotalCheckins        int64   `json:"total_checkins"`
	UniqueUsers          int64   `json:"unique_users"`
	TodayRewardTotal     float64 `json:"today_reward_total"`
	YesterdayRewardTotal float64 `json:"yesterday_reward_total"`
	Last7DaysRewardTotal float64 `json:"last_7_days_reward_total"`
	TotalReward          float64 `json:"total_reward"`
	TotalBonusReward     float64 `json:"total_bonus_reward"`
	TodayBonusCount      int64   `json:"today_bonus_count"`
	TodayDate            string  `json:"today_date"`
	YesterdayDate        string  `json:"yesterday_date"`
	ServerTimezone       string  `json:"server_timezone"`
}

type DailyCheckinService struct {
	repo                 DailyCheckinRepository
	settingService       *SettingService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
	entClient            *dbent.Client
	now                  func() time.Time
}

func NewDailyCheckinService(
	repo DailyCheckinRepository,
	settingService *SettingService,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCacheService *BillingCacheService,
	entClient *dbent.Client,
) *DailyCheckinService {
	return &DailyCheckinService{
		repo:                 repo,
		settingService:       settingService,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
		entClient:            entClient,
		now:                  timezone.Now,
	}
}

func (s *DailyCheckinService) GetStatus(ctx context.Context, userID int64) (*DailyCheckinStatus, error) {
	enabled, dailyReward, weeklyBonus, err := s.config(ctx)
	if err != nil {
		return nil, err
	}

	role, err := s.repo.GetUserRole(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get daily check-in user: %w", err)
	}
	if role != RoleUser {
		enabled = false
	}

	today := timezone.StartOfDay(s.now())
	todayRecord, err := s.repo.GetByDate(ctx, userID, today)
	if err != nil && !errors.Is(err, ErrDailyCheckinNotFound) {
		return nil, fmt.Errorf("get today's daily check-in: %w", err)
	}

	checkedInToday := todayRecord != nil
	currentStreak := 0
	var todayReward *float64
	if checkedInToday {
		currentStreak = todayRecord.StreakCount
		reward := todayRecord.TotalReward
		todayReward = &reward
	} else {
		yesterdayRecord, getErr := s.repo.GetByDate(ctx, userID, today.AddDate(0, 0, -1))
		if getErr != nil && !errors.Is(getErr, ErrDailyCheckinNotFound) {
			return nil, fmt.Errorf("get previous daily check-in: %w", getErr)
		}
		if yesterdayRecord != nil {
			currentStreak = yesterdayRecord.StreakCount
		}
	}

	return &DailyCheckinStatus{
		Enabled:        enabled,
		CheckedInToday: checkedInToday,
		DailyReward:    dailyReward,
		WeeklyBonus:    weeklyBonus,
		CurrentStreak:  currentStreak,
		DaysUntilBonus: daysUntilDailyCheckinBonus(currentStreak),
		TodayReward:    todayReward,
		ServerTimezone: timezone.Name(),
	}, nil
}

// GetHistory returns recent check-in records for the current user.
func (s *DailyCheckinService) GetHistory(ctx context.Context, userID int64, limit int) ([]DailyCheckinHistoryItem, error) {
	role, err := s.repo.GetUserRole(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get daily check-in user: %w", err)
	}
	if role != RoleUser {
		return nil, ErrDailyCheckinRole
	}

	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	records, err := s.repo.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list daily check-in history: %w", err)
	}

	out := make([]DailyCheckinHistoryItem, 0, len(records))
	for i := range records {
		out = append(out, dailyCheckinHistoryItemFromRecord(&records[i]))
	}
	return out, nil
}

// AdminList returns paginated check-in records for administrators.
func (s *DailyCheckinService) AdminList(ctx context.Context, filter DailyCheckinListFilter) ([]DailyCheckinAdminItem, int64, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin daily check-ins: %w", err)
	}

	out := make([]DailyCheckinAdminItem, 0, len(items))
	for i := range items {
		out = append(out, dailyCheckinAdminItemFromListItem(&items[i]))
	}
	return out, total, nil
}

// AdminStats returns aggregate check-in metrics for the admin dashboard.
func (s *DailyCheckinService) AdminStats(ctx context.Context) (*DailyCheckinAdminStats, error) {
	today := timezone.StartOfDay(s.now())
	yesterday := today.AddDate(0, 0, -1)
	last7Start := today.AddDate(0, 0, -6)

	stats, err := s.repo.AdminStats(ctx, today, yesterday, last7Start)
	if err != nil {
		return nil, fmt.Errorf("get admin daily check-in stats: %w", err)
	}
	stats.TodayDate = today.Format("2006-01-02")
	stats.YesterdayDate = yesterday.Format("2006-01-02")
	stats.ServerTimezone = timezone.Name()
	return stats, nil
}

func (s *DailyCheckinService) Checkin(ctx context.Context, userID int64) (*DailyCheckinResult, error) {
	enabled, dailyReward, weeklyBonus, err := s.config(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrDailyCheckinDisabled
	}

	role, err := s.repo.GetUserRole(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get daily check-in user: %w", err)
	}
	if role != RoleUser {
		return nil, ErrDailyCheckinRole
	}

	now := s.now()
	today := timezone.StartOfDay(now)
	record, newBalance, err := s.apply(ctx, userID, today, dailyReward, weeklyBonus)
	if err != nil {
		return nil, err
	}

	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dailyCheckinCacheTimeout)
	defer cancel()
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, userID)
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(cacheCtx, userID)
	}

	return &DailyCheckinResult{
		RewardAmount:  record.TotalReward,
		BaseReward:    record.BaseReward,
		BonusReward:   record.BonusReward,
		NewBalance:    newBalance,
		CurrentStreak: record.StreakCount,
		CheckedInAt:   now.Format(time.RFC3339),
	}, nil
}

func (s *DailyCheckinService) apply(
	ctx context.Context,
	userID int64,
	today time.Time,
	dailyReward float64,
	weeklyBonus float64,
) (*DailyCheckinRecord, float64, error) {
	apply := func(opCtx context.Context) (*DailyCheckinRecord, float64, error) {
		if err := s.repo.LockUserForCheckin(opCtx, userID); err != nil {
			return nil, 0, fmt.Errorf("lock daily check-in user: %w", err)
		}
		yesterday, err := s.repo.GetByDate(opCtx, userID, today.AddDate(0, 0, -1))
		if err != nil && !errors.Is(err, ErrDailyCheckinNotFound) {
			return nil, 0, fmt.Errorf("get previous daily check-in: %w", err)
		}

		streak := 1
		if yesterday != nil {
			streak = yesterday.StreakCount + 1
		}
		bonusReward := 0.0
		if streak%dailyCheckinBonusInterval == 0 {
			bonusReward = weeklyBonus
		}
		record := &DailyCheckinRecord{
			UserID:      userID,
			CheckinDate: today,
			BaseReward:  dailyReward,
			BonusReward: bonusReward,
			TotalReward: roundDailyCheckinAmount(dailyReward + bonusReward),
			StreakCount: streak,
		}

		if err := s.repo.Create(opCtx, record); err != nil {
			return nil, 0, err
		}
		balance, err := s.repo.AddUserBalance(opCtx, record.UserID, record.TotalReward)
		if err != nil {
			return nil, 0, fmt.Errorf("add daily check-in balance: %w", err)
		}
		return record, balance, nil
	}
	if s.entClient == nil {
		return apply(ctx)
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("begin daily check-in transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	record, balance, err := apply(dbent.NewTxContext(ctx, tx))
	if err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("commit daily check-in transaction: %w", err)
	}
	return record, balance, nil
}

func (s *DailyCheckinService) config(ctx context.Context) (bool, float64, float64, error) {
	if s.settingService == nil {
		return false, DailyCheckinRewardDefault, DailyCheckinWeeklyBonusDefault, nil
	}
	settings, err := s.settingService.GetAllSettings(ctx)
	if err != nil {
		return false, 0, 0, fmt.Errorf("get daily check-in settings: %w", err)
	}
	return settings.DailyCheckinEnabled,
		normalizeDailyCheckinAmount(settings.DailyCheckinReward, DailyCheckinRewardDefault),
		normalizeDailyCheckinAmount(settings.DailyCheckinWeeklyBonus, DailyCheckinWeeklyBonusDefault),
		nil
}

func normalizeDailyCheckinAmount(value, fallback float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return fallback
	}
	if value > DailyCheckinRewardLimit {
		return DailyCheckinRewardLimit
	}
	return roundDailyCheckinAmount(value)
}

func roundDailyCheckinAmount(value float64) float64 {
	return math.Round(value*1e8) / 1e8
}

func daysUntilDailyCheckinBonus(streak int) int {
	if streak <= 0 {
		return dailyCheckinBonusInterval
	}
	remainder := streak % dailyCheckinBonusInterval
	if remainder == 0 {
		return dailyCheckinBonusInterval
	}
	return dailyCheckinBonusInterval - remainder
}

func dailyCheckinHistoryItemFromRecord(record *DailyCheckinRecord) DailyCheckinHistoryItem {
	return DailyCheckinHistoryItem{
		ID:          record.ID,
		CheckinDate: record.CheckinDate.Format("2006-01-02"),
		BaseReward:  record.BaseReward,
		BonusReward: record.BonusReward,
		TotalReward: record.TotalReward,
		StreakCount: record.StreakCount,
		CreatedAt:   record.CreatedAt.Format(time.RFC3339),
	}
}

func dailyCheckinAdminItemFromListItem(item *DailyCheckinListItem) DailyCheckinAdminItem {
	return DailyCheckinAdminItem{
		ID:          item.ID,
		UserID:      item.UserID,
		UserEmail:   item.UserEmail,
		Username:    item.Username,
		CheckinDate: item.CheckinDate.Format("2006-01-02"),
		BaseReward:  item.BaseReward,
		BonusReward: item.BonusReward,
		TotalReward: item.TotalReward,
		StreakCount: item.StreakCount,
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
	}
}
