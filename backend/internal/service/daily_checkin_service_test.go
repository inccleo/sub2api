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
	svc := NewDailyCheckinService(repo, settings, nil, nil, nil)

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
	svc := NewDailyCheckinService(repo, newDailyCheckinTestSettings(true, 0.25, 1), nil, nil, nil)
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
	svc := NewDailyCheckinService(repo, newDailyCheckinTestSettings(true, 0.10, 0.50), nil, nil, nil)
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
	svc := NewDailyCheckinService(repo, newDailyCheckinTestSettings(false, 0.10, 0.50), nil, nil, nil)

	_, err := svc.Checkin(context.Background(), 1)
	require.ErrorIs(t, err, ErrDailyCheckinDisabled)
	require.Empty(t, repo.records)
}

func TestDailyCheckinServiceRejectsNonUserRole(t *testing.T) {
	repo := newDailyCheckinTestRepo()
	repo.role = RoleAdmin
	svc := NewDailyCheckinService(
		repo,
		newDailyCheckinTestSettings(true, 0.10, 0.50),
		nil,
		nil,
		nil,
	)

	_, err := svc.Checkin(context.Background(), 1)
	require.ErrorIs(t, err, ErrDailyCheckinRole)
	require.Empty(t, repo.records)
}

func TestDailyCheckinServiceStatusBeforeAndAfterCheckin(t *testing.T) {
	repo := newDailyCheckinTestRepo()
	svc := NewDailyCheckinService(
		repo,
		newDailyCheckinTestSettings(true, 0.10, 0.50),
		nil,
		nil,
		nil,
	)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, timezone.Location())
	svc.now = func() time.Time { return now }

	before, err := svc.GetStatus(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, before.Enabled)
	require.False(t, before.CheckedInToday)
	require.Zero(t, before.CurrentStreak)
	require.Equal(t, 7, before.DaysUntilBonus)
	require.Nil(t, before.TodayReward)

	_, err = svc.Checkin(context.Background(), 1)
	require.NoError(t, err)
	after, err := svc.GetStatus(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, after.CheckedInToday)
	require.Equal(t, 1, after.CurrentStreak)
	require.Equal(t, 6, after.DaysUntilBonus)
	require.NotNil(t, after.TodayReward)
	require.InDelta(t, 0.10, *after.TodayReward, 1e-9)
}

func TestDailyCheckinServiceStatusShowsProgressFromYesterday(t *testing.T) {
	repo := newDailyCheckinTestRepo()
	svc := NewDailyCheckinService(
		repo,
		newDailyCheckinTestSettings(true, 0.10, 0.50),
		nil,
		nil,
		nil,
	)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, timezone.Location())
	svc.now = func() time.Time { return now }
	yesterday := timezone.StartOfDay(now).AddDate(0, 0, -1)
	repo.records[dailyCheckinTestKey(1, yesterday)] = &DailyCheckinRecord{
		UserID:      1,
		CheckinDate: yesterday,
		StreakCount: 6,
	}

	got, err := svc.GetStatus(context.Background(), 1)
	require.NoError(t, err)
	require.False(t, got.CheckedInToday)
	require.Equal(t, 6, got.CurrentStreak)
	require.Equal(t, 1, got.DaysUntilBonus)
}

func TestDailyCheckinServiceInvalidatesAPIKeyAuthCache(t *testing.T) {
	repo := newDailyCheckinTestRepo()
	authCache := &dailyCheckinAuthCacheStub{}
	svc := NewDailyCheckinService(
		repo,
		newDailyCheckinTestSettings(true, 0.10, 0.50),
		authCache,
		nil,
		nil,
	)

	_, err := svc.Checkin(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, []int64{42}, authCache.userIDs)
}

func TestDailyCheckinServiceInvalidatesCacheAfterRequestCancellation(t *testing.T) {
	repo := newDailyCheckinTestRepo()
	authCache := &dailyCheckinAuthCacheStub{}
	svc := NewDailyCheckinService(
		repo,
		newDailyCheckinTestSettings(true, 0.10, 0.50),
		authCache,
		nil,
		nil,
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Checkin(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, []int64{42}, authCache.userIDs)
	require.NoError(t, authCache.contextErr)
}

type dailyCheckinAuthCacheStub struct {
	userIDs    []int64
	contextErr error
}

func (*dailyCheckinAuthCacheStub) InvalidateAuthCacheByKey(context.Context, string) {}

func (s *dailyCheckinAuthCacheStub) InvalidateAuthCacheByUserID(ctx context.Context, userID int64) {
	s.userIDs = append(s.userIDs, userID)
	s.contextErr = ctx.Err()
}

func (*dailyCheckinAuthCacheStub) InvalidateAuthCacheByGroupID(context.Context, int64) {}

type dailyCheckinTestRepo struct {
	records map[string]*DailyCheckinRecord
	balance float64
	role    string
}

func newDailyCheckinTestRepo() *dailyCheckinTestRepo {
	return &dailyCheckinTestRepo{records: make(map[string]*DailyCheckinRecord)}
}

func (r *dailyCheckinTestRepo) GetUserRole(context.Context, int64) (string, error) {
	if r.role == "" {
		return RoleUser, nil
	}
	return r.role, nil
}

func (r *dailyCheckinTestRepo) GetByDate(_ context.Context, userID int64, date time.Time) (*DailyCheckinRecord, error) {
	record, ok := r.records[dailyCheckinTestKey(userID, date)]
	if !ok {
		return nil, ErrDailyCheckinNotFound
	}
	copyRecord := *record
	return &copyRecord, nil
}

func (*dailyCheckinTestRepo) LockUserForCheckin(context.Context, int64) error {
	return nil
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
