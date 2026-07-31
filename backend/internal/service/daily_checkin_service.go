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

type DailyCheckinRepository interface {
	GetUserRole(ctx context.Context, userID int64) (string, error)
	GetByDate(ctx context.Context, userID int64, date time.Time) (*DailyCheckinRecord, error)
	LockUserForCheckin(ctx context.Context, userID int64) error
	Create(ctx context.Context, record *DailyCheckinRecord) error
	AddUserBalance(ctx context.Context, userID int64, amount float64) (float64, error)
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
