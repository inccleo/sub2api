package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func TestDailyCheckinServiceSeventhDayBonus(t *testing.T) {
	repo := newDailyCheckinTestRepo()
	settings := newDailyCheckinTestSettings(true, 0.10, 0.50)
	svc := NewDailyCheckinService(repo, settings, nil, nil)

	now := time.Date(2026, 7, 1, 8, 0, 0, 0, timezone.Location())
	svc.now = func() time.Time { return now }

	for day := 1; day <= 7; day++ {
		result, err := svc.Checkin(context.Background(), 1)
		require.NoError(t, err)
		require.Equal(t, day, result.CurrentStreak)
		if day < 7 {
			require.InDelta(t, 0.10, result.RewardAmount, 1e-9)
			require.Zero(t, result.BonusReward)
		} else {
			require.InDelta(t, 0.60, result.RewardAmount, 1e-9)
			require.InDelta(t, 0.50, result.BonusReward, 1e-9)
		}
		now = now.AddDate(0, 0, 1)
	}
	require.InDelta(t, 1.20, repo.balance, 1e-9)
}

func TestDailyCheckinServiceRejectsDuplicate(t *testing.T) {
	repo := newDailyCheckinTestRepo()
	svc := NewDailyCheckinService(repo, newDailyCheckinTestSettings(true, 0.25, 1), nil, nil)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, timezone.Location())
	svc.now = func() time.Time { return now }

	first, err := svc.Checkin(context.Background(), 1)
	require.NoError(t, err)
	_, err = svc.Checkin(context.Background(), 1)
	require.ErrorIs(t, err, ErrDailyCheckinAlreadyDone)
	require.InDelta(t, first.NewBalance, repo.balance, 1e-9)
}

func TestDailyCheckinServiceResetsBrokenStreak(t *testing.T) {
	repo := newDailyCheckinTestRepo()
	svc := NewDailyCheckinService(repo, newDailyCheckinTestSettings(true, 0.10, 0.50), nil, nil)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, timezone.Location())
	svc.now = func() time.Time { return now }

	_, err := svc.Checkin(context.Background(), 1)
	require.NoError(t, err)
	now = now.AddDate(0, 0, 2)
	result, err := svc.Checkin(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, result.CurrentStreak)
	require.Zero(t, result.BonusReward)
}

func TestDailyCheckinServiceDisabled(t *testing.T) {
	repo := newDailyCheckinTestRepo()
	svc := NewDailyCheckinService(repo, newDailyCheckinTestSettings(false, 0.10, 0.50), nil, nil)

	_, err := svc.Checkin(context.Background(), 1)
	require.ErrorIs(t, err, ErrDailyCheckinDisabled)
	require.Empty(t, repo.records)
}

type dailyCheckinTestRepo struct {
	records map[string]*DailyCheckinRecord
	balance float64
}

func newDailyCheckinTestRepo() *dailyCheckinTestRepo {
	return &dailyCheckinTestRepo{records: make(map[string]*DailyCheckinRecord)}
}

func (r *dailyCheckinTestRepo) GetUserRole(context.Context, int64) (string, error) {
	return RoleUser, nil
}

func (r *dailyCheckinTestRepo) GetByDate(_ context.Context, userID int64, date time.Time) (*DailyCheckinRecord, error) {
	record, ok := r.records[dailyCheckinTestKey(userID, date)]
	if !ok {
		return nil, ErrDailyCheckinNotFound
	}
	copyRecord := *record
	return &copyRecord, nil
}

func (r *dailyCheckinTestRepo) Create(_ context.Context, record *DailyCheckinRecord) error {
	key := dailyCheckinTestKey(record.UserID, record.CheckinDate)
	if _, exists := r.records[key]; exists {
		return ErrDailyCheckinAlreadyDone
	}
	copyRecord := *record
	r.records[key] = &copyRecord
	return nil
}

func (r *dailyCheckinTestRepo) AddUserBalance(_ context.Context, _ int64, amount float64) (float64, error) {
	r.balance += amount
	return r.balance, nil
}

func dailyCheckinTestKey(userID int64, date time.Time) string {
	return strconv.FormatInt(userID, 10) + ":" + date.Format("2006-01-02")
}

type dailyCheckinTestSettingRepo struct {
	values map[string]string
}

func newDailyCheckinTestSettings(enabled bool, reward, bonus float64) *SettingService {
	repo := &dailyCheckinTestSettingRepo{values: map[string]string{
		SettingKeyDailyCheckinEnabled:     strconv.FormatBool(enabled),
		SettingKeyDailyCheckinReward:      strconv.FormatFloat(reward, 'f', 8, 64),
		SettingKeyDailyCheckinWeeklyBonus: strconv.FormatFloat(bonus, 'f', 8, 64),
	}}
	return NewSettingService(repo, &config.Config{
		Default: config.DefaultConfig{UserConcurrency: 1},
	})
}

func (r *dailyCheckinTestSettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	value, ok := r.values[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *dailyCheckinTestSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *dailyCheckinTestSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *dailyCheckinTestSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *dailyCheckinTestSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *dailyCheckinTestSettingRepo) GetAll(context.Context) (map[string]string, error) {
	result := make(map[string]string, len(r.values))
	for key, value := range r.values {
		result[key] = value
	}
	return result, nil
}

func (r *dailyCheckinTestSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

var _ SettingRepository = (*dailyCheckinTestSettingRepo)(nil)
var _ DailyCheckinRepository = (*dailyCheckinTestRepo)(nil)
