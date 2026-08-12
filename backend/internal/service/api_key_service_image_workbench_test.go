//go:build unit

package service

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type imageWorkbenchAPIKeyRepoStub struct {
	quotaBaseAPIKeyRepoStub

	mu          sync.Mutex
	nextID      int64
	keys        map[int64]APIKey
	createCalls int
}

func newImageWorkbenchAPIKeyRepoStub(keys ...APIKey) *imageWorkbenchAPIKeyRepoStub {
	repo := &imageWorkbenchAPIKeyRepoStub{nextID: 1, keys: make(map[int64]APIKey)}
	for _, key := range keys {
		if key.ID == 0 {
			key.ID = repo.nextID
		}
		if key.ID >= repo.nextID {
			repo.nextID = key.ID + 1
		}
		repo.keys[key.ID] = key
	}
	return repo
}

func (s *imageWorkbenchAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key.ID = s.nextID
	s.nextID++
	s.keys[key.ID] = *key
	s.createCalls++
	return nil
}

func (s *imageWorkbenchAPIKeyRepoStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, ok := s.keys[id]
	if !ok {
		return nil, ErrAPIKeyNotFound
	}
	return cloneImageWorkbenchAPIKey(key), nil
}

func (s *imageWorkbenchAPIKeyRepoStub) Update(_ context.Context, key *APIKey, _ APIKeyUpdateFields) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[key.ID] = *key
	return nil
}

func (s *imageWorkbenchAPIKeyRepoStub) ExistsByKey(_ context.Context, credential string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range s.keys {
		if key.Key == credential {
			return true, nil
		}
	}
	return false, nil
}

func (s *imageWorkbenchAPIKeyRepoStub) SearchAPIKeys(_ context.Context, userID int64, keyword string, limit int) ([]APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]APIKey, 0)
	for _, key := range s.keys {
		if key.UserID != userID || (!strings.Contains(key.Name, keyword) && !strings.Contains(key.Key, keyword)) {
			continue
		}
		keys = append(keys, *cloneImageWorkbenchAPIKey(key))
		if limit > 0 && len(keys) >= limit {
			break
		}
	}
	return keys, nil
}

func (s *imageWorkbenchAPIKeyRepoStub) snapshot() ([]APIKey, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]APIKey, 0, len(s.keys))
	for _, key := range s.keys {
		keys = append(keys, *cloneImageWorkbenchAPIKey(key))
	}
	return keys, s.createCalls
}

func cloneImageWorkbenchAPIKey(key APIKey) *APIKey {
	clone := key
	if key.GroupID != nil {
		groupID := *key.GroupID
		clone.GroupID = &groupID
	}
	return &clone
}

type imageWorkbenchGroupRepoStub struct {
	groupRepoNoop
	groups []Group
}

func (s *imageWorkbenchGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	for i := range s.groups {
		if s.groups[i].ID == id {
			group := s.groups[i]
			return &group, nil
		}
	}
	return nil, ErrGroupNotFound
}

func (s *imageWorkbenchGroupRepoStub) ListActive(context.Context) ([]Group, error) {
	return append([]Group(nil), s.groups...), nil
}

type imageWorkbenchUserSubRepoStub struct{ userSubRepoNoop }

func (imageWorkbenchUserSubRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}

func newImageWorkbenchAPIKeyService(repo *imageWorkbenchAPIKeyRepoStub, groups ...Group) *APIKeyService {
	const userID int64 = 7
	return NewAPIKeyService(
		repo,
		&mockUserRepo{getByIDUser: &User{ID: userID, Status: StatusActive}},
		&imageWorkbenchGroupRepoStub{groups: groups},
		imageWorkbenchUserSubRepoStub{},
		nil,
		nil,
		&config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-"}},
	)
}

func availableImageWorkbenchGroup(id int64) Group {
	return Group{
		ID:                   id,
		Platform:             PlatformOpenAI,
		Status:               StatusActive,
		SubscriptionType:     SubscriptionTypeStandard,
		AllowImageGeneration: true,
	}
}

func TestGetOrCreateImageWorkbenchKeyReusesManagedKey(t *testing.T) {
	groupID := int64(12)
	existing := APIKey{
		ID:      41,
		UserID:  7,
		Key:     "sk-iwb-existing-managed-credential",
		Name:    ImageWorkbenchAPIKeyName,
		GroupID: &groupID,
		Status:  StatusActive,
	}
	repo := newImageWorkbenchAPIKeyRepoStub(existing)
	svc := newImageWorkbenchAPIKeyService(repo, availableImageWorkbenchGroup(groupID))

	key, err := svc.GetOrCreateImageWorkbenchKey(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, existing.ID, key.ID)
	_, createCalls := repo.snapshot()
	require.Zero(t, createCalls)
}

func TestGetOrCreateImageWorkbenchKeyRenamesLegacyKeyBeforeCreatingManagedKey(t *testing.T) {
	groupID := int64(12)
	legacy := APIKey{
		ID:      41,
		UserID:  7,
		Key:     "sk-user-known-legacy-credential",
		Name:    ImageWorkbenchAPIKeyName,
		GroupID: &groupID,
		Status:  StatusActive,
	}
	repo := newImageWorkbenchAPIKeyRepoStub(legacy)
	svc := newImageWorkbenchAPIKeyService(repo, availableImageWorkbenchGroup(groupID))

	managed, err := svc.GetOrCreateImageWorkbenchKey(context.Background(), 7)
	require.NoError(t, err)
	require.NotEqual(t, legacy.ID, managed.ID)
	require.Equal(t, ImageWorkbenchAPIKeyName, managed.Name)
	require.True(t, strings.HasPrefix(managed.Key, "sk-iwb-"))

	keys, createCalls := repo.snapshot()
	require.Equal(t, 1, createCalls)
	require.Len(t, keys, 2)
	for _, key := range keys {
		if key.ID == legacy.ID {
			require.Equal(t, "image-workbench-legacy-41", key.Name)
			require.Equal(t, legacy.Key, key.Key)
		}
	}
}

func TestGetOrCreateImageWorkbenchKeyReturnsUnavailableWithoutEligibleGroup(t *testing.T) {
	repo := newImageWorkbenchAPIKeyRepoStub()
	svc := newImageWorkbenchAPIKeyService(repo, Group{
		ID:                   12,
		Platform:             PlatformAnthropic,
		Status:               StatusActive,
		SubscriptionType:     SubscriptionTypeStandard,
		AllowImageGeneration: true,
	})

	key, err := svc.GetOrCreateImageWorkbenchKey(context.Background(), 7)
	require.Nil(t, key)
	require.ErrorIs(t, err, ErrImageWorkbenchUnavailable)
	_, createCalls := repo.snapshot()
	require.Zero(t, createCalls)
}

func TestGetOrCreateImageWorkbenchKeyConcurrentFirstUseCreatesOnce(t *testing.T) {
	repo := newImageWorkbenchAPIKeyRepoStub()
	svc := newImageWorkbenchAPIKeyService(repo, availableImageWorkbenchGroup(12))

	const callers = 32
	start := make(chan struct{})
	results := make(chan *APIKey, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			key, err := svc.GetOrCreateImageWorkbenchKey(context.Background(), 7)
			results <- key
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	var managedID int64
	for key := range results {
		require.NotNil(t, key)
		if managedID == 0 {
			managedID = key.ID
		}
		require.Equal(t, managedID, key.ID)
	}
	keys, createCalls := repo.snapshot()
	require.Equal(t, 1, createCalls)
	require.Len(t, keys, 1)
}
