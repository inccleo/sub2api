package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"regexp"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DesktopClientID    = "sub2api-desktop"
	DesktopRedirectURI = "sub2api://oauth/callback"
	DesktopAuthCodeTTL = 60 * time.Second
)

var (
	ErrDesktopInvalidGrant = infraerrors.BadRequest("invalid_grant", "Authorization code is invalid, expired, or already used")
	desktopVerifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	desktopStatePattern    = regexp.MustCompile(`^[A-Za-z0-9_-]{32,128}$`)
)

// DesktopAuthCodeStore must validate the binding and delete the code atomically.
// Implementations store only the SHA-256 hash of the authorization code.
type DesktopAuthCodeStore interface {
	Store(context.Context, string, *DesktopAuthGrant, time.Duration) error
	Consume(context.Context, string, string, string, string) (*DesktopAuthGrant, error)
}

type DesktopAuthGrant struct {
	UserID       int64     `json:"user_id"`
	TokenVersion int64     `json:"token_version"`
	ClientID     string    `json:"client_id"`
	RedirectURI  string    `json:"redirect_uri"`
	Challenge    string    `json:"challenge"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type DesktopAuthorizeRequest struct {
	ClientID            string `json:"client_id" form:"client_id"`
	RedirectURI         string `json:"redirect_uri" form:"redirect_uri"`
	ResponseType        string `json:"response_type" form:"response_type"`
	Scope               string `json:"scope" form:"scope"`
	State               string `json:"state" form:"state"`
	CodeChallenge       string `json:"code_challenge" form:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method" form:"code_challenge_method"`
}

type DesktopTokenRequest struct {
	GrantType    string `json:"grant_type" form:"grant_type"`
	ClientID     string `json:"client_id" form:"client_id"`
	RedirectURI  string `json:"redirect_uri" form:"redirect_uri"`
	Code         string `json:"code" form:"code"`
	CodeVerifier string `json:"code_verifier" form:"code_verifier"`
}

// This is a first-party account login, not a general third-party OAuth provider.
// The issued session has the same permissions as a browser login.
type DesktopAuthService struct {
	cfg   *config.Config
	store DesktopAuthCodeStore
	auth  *AuthService
}

func NewDesktopAuthService(cfg *config.Config, store DesktopAuthCodeStore, auth *AuthService) *DesktopAuthService {
	return &DesktopAuthService{cfg: cfg, store: store, auth: auth}
}

func (s *DesktopAuthService) enabled() error {
	if s == nil || s.cfg == nil || !s.cfg.DesktopAuth.Enabled {
		return infraerrors.NotFound("desktop_auth_disabled", "Desktop authorization is disabled")
	}
	return nil
}

func (s *DesktopAuthService) Validate(req DesktopAuthorizeRequest) error {
	if err := s.enabled(); err != nil {
		return err
	}
	if req.ClientID != DesktopClientID || req.RedirectURI != DesktopRedirectURI {
		return infraerrors.BadRequest("invalid_client", "Unknown desktop client or redirect URI")
	}
	if req.ResponseType != "code" || req.Scope != "account" || req.CodeChallengeMethod != "S256" || !desktopStatePattern.MatchString(req.State) || !desktopSHA256Value(req.CodeChallenge) {
		return infraerrors.BadRequest("invalid_request", "Expected response_type=code, scope=account, a random state, and S256 PKCE")
	}
	return nil
}

func desktopSHA256Value(value string) bool {
	b, err := base64.RawURLEncoding.Strict().DecodeString(value)
	return err == nil && len(b) == sha256.Size && len(value) == 43
}

func (s *DesktopAuthService) Describe(ctx context.Context, userID int64, req DesktopAuthorizeRequest) (*User, error) {
	if err := s.Validate(req); err != nil {
		return nil, err
	}
	return s.loginUser(ctx, userID)
}

func (s *DesktopAuthService) loginUser(ctx context.Context, userID int64) (*User, error) {
	user, err := s.auth.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.IsActive() || user.DeletedAt != nil {
		return nil, ErrUserNotActive
	}
	if s.auth.settingService != nil && s.auth.settingService.IsBackendModeEnabled(ctx) && !user.IsAdmin() {
		return nil, infraerrors.Forbidden("BACKEND_MODE_ADMIN_ONLY", "Backend mode is active. Only admin login is allowed.")
	}
	return user, nil
}

// Authorize is called only after an authenticated browser explicitly approves or denies.
func (s *DesktopAuthService) Authorize(ctx context.Context, userID int64, req DesktopAuthorizeRequest, approve bool) (string, error) {
	if err := s.Validate(req); err != nil {
		return "", err
	}
	u, _ := url.Parse(DesktopRedirectURI)
	query := url.Values{"state": {req.State}}
	if !approve {
		query.Set("error", "access_denied")
	} else {
		user, err := s.loginUser(ctx, userID)
		if err != nil {
			return "", err
		}
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return "", ErrServiceUnavailable
		}
		code := base64.RawURLEncoding.EncodeToString(raw[:])
		grant := &DesktopAuthGrant{UserID: user.ID, TokenVersion: resolvedTokenVersion(user), ClientID: req.ClientID, RedirectURI: req.RedirectURI, Challenge: req.CodeChallenge, ExpiresAt: time.Now().Add(DesktopAuthCodeTTL)}
		if err := s.store.Store(ctx, hashToken(code), grant, DesktopAuthCodeTTL); err != nil {
			return "", ErrServiceUnavailable
		}
		query.Set("code", code)
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func (s *DesktopAuthService) Exchange(ctx context.Context, req DesktopTokenRequest) (*TokenPair, *User, error) {
	if err := s.enabled(); err != nil {
		return nil, nil, err
	}
	if req.GrantType != "authorization_code" {
		return nil, nil, infraerrors.BadRequest("unsupported_grant_type", "Expected authorization_code")
	}
	if req.ClientID != DesktopClientID || req.RedirectURI != DesktopRedirectURI || !desktopSHA256Value(req.Code) || !desktopVerifierPattern.MatchString(req.CodeVerifier) {
		return nil, nil, ErrDesktopInvalidGrant
	}
	sum := sha256.Sum256([]byte(req.CodeVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	grant, err := s.store.Consume(ctx, hashToken(req.Code), req.ClientID, req.RedirectURI, challenge)
	if err != nil {
		return nil, nil, err
	}
	if grant == nil || !time.Now().Before(grant.ExpiresAt) {
		return nil, nil, ErrDesktopInvalidGrant
	}
	user, err := s.loginUser(ctx, grant.UserID)
	if err != nil {
		return nil, nil, err
	}
	if resolvedTokenVersion(user) != grant.TokenVersion {
		return nil, nil, ErrDesktopInvalidGrant
	}
	// Issue a separate refresh-token family, bound to the desktop request (not the browser).
	pair, err := s.auth.GenerateTokenPair(ctx, user, "")
	if err != nil {
		return nil, nil, ErrServiceUnavailable
	}
	s.auth.RecordSuccessfulLogin(ctx, user.ID)
	return pair, user, nil
}
