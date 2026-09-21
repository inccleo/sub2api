package handler

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// imageWorkbenchUpstream is the chatgpt2api endpoint that owns the image model
// catalog for an image-generation group.
type imageWorkbenchUpstream struct {
	BaseURL string
	APIKey  string
}

// imageWorkbenchUpstreamResolver finds the upstream backing an image group.
type imageWorkbenchUpstreamResolver interface {
	ResolveImageWorkbenchUpstream(ctx context.Context, groupID int64) (*imageWorkbenchUpstream, error)
}

// accountImageWorkbenchUpstreamResolver reads the upstream from the schedulable
// OpenAI account bound to the caller's image group. Reusing the generation
// account keeps the model picker aligned with what can actually run and avoids
// duplicating the downstream credential in configuration.
type accountImageWorkbenchUpstreamResolver struct {
	accounts service.AccountRepository
}

func newAccountImageWorkbenchUpstreamResolver(accounts service.AccountRepository) *accountImageWorkbenchUpstreamResolver {
	return &accountImageWorkbenchUpstreamResolver{accounts: accounts}
}

func (r *accountImageWorkbenchUpstreamResolver) ResolveImageWorkbenchUpstream(ctx context.Context, groupID int64) (*imageWorkbenchUpstream, error) {
	if r == nil || r.accounts == nil || groupID <= 0 {
		return nil, service.ErrImageWorkbenchUnavailable
	}
	accounts, err := r.accounts.ListSchedulableByGroupIDAndPlatform(ctx, groupID, service.PlatformOpenAI)
	if err != nil {
		return nil, err
	}
	// Only custom-endpoint accounts carry a base_url, so this skips official
	// upstreams without needing to special-case account types.
	for i := range accounts {
		base := strings.TrimRight(strings.TrimSpace(accounts[i].GetCredential("base_url")), "/")
		if base == "" {
			continue
		}
		return &imageWorkbenchUpstream{
			BaseURL: base,
			APIKey:  strings.TrimSpace(accounts[i].GetCredential("api_key")),
		}, nil
	}
	return nil, service.ErrImageWorkbenchUnavailable
}
