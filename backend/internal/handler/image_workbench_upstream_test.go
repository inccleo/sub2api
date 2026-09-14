package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type imageWorkbenchAccountRepoStub struct {
	service.AccountRepository
	accounts []service.Account
	groupID  int64
	platform string
}

func (s *imageWorkbenchAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]service.Account, error) {
	s.groupID = groupID
	s.platform = platform
	return s.accounts, nil
}

func TestAccountImageWorkbenchUpstreamResolverPicksFirstCustomEndpoint(t *testing.T) {
	repo := &imageWorkbenchAccountRepoStub{accounts: []service.Account{
		{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"api_key": "official"}},
		{ID: 2, Platform: service.PlatformOpenAI, Credentials: map[string]any{"base_url": "http://chatgpt2api.example.internal:3002/", "api_key": " downstream-key "}},
	}}
	resolver := newAccountImageWorkbenchUpstreamResolver(repo)

	upstream, err := resolver.ResolveImageWorkbenchUpstream(context.Background(), 2)

	require.NoError(t, err)
	require.Equal(t, int64(2), repo.groupID)
	require.Equal(t, service.PlatformOpenAI, repo.platform)
	require.Equal(t, "http://chatgpt2api.example.internal:3002", upstream.BaseURL)
	require.Equal(t, "downstream-key", upstream.APIKey)
}

func TestAccountImageWorkbenchUpstreamResolverRequiresCustomEndpoint(t *testing.T) {
	resolver := newAccountImageWorkbenchUpstreamResolver(&imageWorkbenchAccountRepoStub{accounts: []service.Account{
		{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"api_key": "official"}},
	}})

	_, err := resolver.ResolveImageWorkbenchUpstream(context.Background(), 2)

	require.ErrorIs(t, err, service.ErrImageWorkbenchUnavailable)
}

func TestAccountImageWorkbenchUpstreamResolverRejectsMissingGroup(t *testing.T) {
	resolver := newAccountImageWorkbenchUpstreamResolver(&imageWorkbenchAccountRepoStub{})

	_, err := resolver.ResolveImageWorkbenchUpstream(context.Background(), 0)

	require.ErrorIs(t, err, service.ErrImageWorkbenchUnavailable)
}
