package routes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type desktopUserRepo struct {
	service.UserRepository
	user *service.User
}

func (r *desktopUserRepo) GetByID(context.Context, int64) (*service.User, error) { return r.user, nil }

type desktopFixture struct {
	router   *gin.Engine
	redis    *miniredis.Miniredis
	user     *service.User
	cfg      *config.Config
	request  service.DesktopAuthorizeRequest
	verifier string
}

func newDesktopFixture(t *testing.T) *desktopFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{DesktopAuth: config.DesktopAuthConfig{Enabled: true}, JWT: config.JWTConfig{Secret: strings.Repeat("s", 32), AccessTokenExpireMinutes: 15, RefreshTokenExpireDays: 7}}
	user := &service.User{ID: 42, Email: "desktop@dingtalk-connect.invalid", Status: service.StatusActive, Role: service.RoleUser, PasswordHash: "initial-hash"}
	auth := service.NewAuthService(nil, &desktopUserRepo{user: user}, nil, repository.NewRefreshTokenCache(rdb), cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	svc := service.NewDesktopAuthService(cfg, repository.NewDesktopAuthCodeStore(rdb), auth)
	h := handler.NewDesktopAuthHandler(svc)
	router := gin.New()
	router.POST("/token", h.Token)
	protected := router.Group("", func(c *gin.Context) {
		if c.GetHeader("Authorization") != "Bearer browser-session" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: user.ID})
	})
	protected.POST("/authorize", h.Authorize)
	protected.GET("/authorize", h.Details)
	verifier := strings.Repeat("v", 43)
	sum := sha256.Sum256([]byte(verifier))
	return &desktopFixture{router: router, redis: mr, user: user, cfg: cfg, verifier: verifier, request: service.DesktopAuthorizeRequest{ClientID: service.DesktopClientID, RedirectURI: service.DesktopRedirectURI, ResponseType: "code", Scope: "account", State: strings.Repeat("s", 43), CodeChallenge: base64.RawURLEncoding.EncodeToString(sum[:]), CodeChallengeMethod: "S256"}}
}

func (f *desktopFixture) post(path string, body any, authenticated bool) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if authenticated {
		req.Header.Set("Authorization", "Bearer browser-session")
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

func (f *desktopFixture) authorize(t *testing.T, decision string) string {
	t.Helper()
	w := f.post("/authorize", struct {
		service.DesktopAuthorizeRequest
		Decision string `json:"decision"`
	}{f.request, decision}, true)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var result struct {
		Data struct {
			RedirectURI string `json:"redirect_uri"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	return result.Data.RedirectURI
}

func (f *desktopFixture) tokenRequest(t *testing.T) service.DesktopTokenRequest {
	t.Helper()
	callback, err := url.Parse(f.authorize(t, "approve"))
	require.NoError(t, err)
	require.Equal(t, f.request.State, callback.Query().Get("state"))
	require.Empty(t, callback.Query().Get("access_token"))
	return service.DesktopTokenRequest{GrantType: "authorization_code", ClientID: service.DesktopClientID, RedirectURI: service.DesktopRedirectURI, Code: callback.Query().Get("code"), CodeVerifier: f.verifier}
}

func TestDesktopAuthorizationExchangeAndReplay(t *testing.T) {
	f := newDesktopFixture(t)
	req := f.tokenRequest(t)
	for _, key := range f.redis.Keys() {
		require.NotContains(t, key, req.Code)
	}
	w := f.post("/token", req, false)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var result struct {
		Data handler.AuthResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.NotEmpty(t, result.Data.AccessToken)
	require.NotEmpty(t, result.Data.RefreshToken)
	require.Equal(t, int64(42), result.Data.User.ID)
	require.Equal(t, 900, result.Data.ExpiresIn)
	w = f.post("/token", req, false)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid_grant")
}

func TestDesktopAuthorizationBindingsDoNotConsumeValidCode(t *testing.T) {
	f := newDesktopFixture(t)
	req := f.tokenRequest(t)
	for _, change := range []func(*service.DesktopTokenRequest){
		func(r *service.DesktopTokenRequest) { r.ClientID = "attacker" },
		func(r *service.DesktopTokenRequest) { r.RedirectURI += "/other" },
		func(r *service.DesktopTokenRequest) { r.CodeVerifier = strings.Repeat("w", 43) },
		func(r *service.DesktopTokenRequest) { r.CodeVerifier = "short" },
		func(r *service.DesktopTokenRequest) { r.GrantType = "password" },
	} {
		bad := req
		change(&bad)
		require.Equal(t, http.StatusBadRequest, f.post("/token", bad, false).Code)
	}
	require.Equal(t, http.StatusOK, f.post("/token", req, false).Code)
}

func TestDesktopAuthorizationConcurrentExchange(t *testing.T) {
	f := newDesktopFixture(t)
	req := f.tokenRequest(t)
	var success atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if f.post("/token", req, false).Code == http.StatusOK {
				success.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), success.Load())
}

func TestDesktopAuthorizationExpiryAndAccountChanges(t *testing.T) {
	for _, name := range []string{"expired", "password_changed", "disabled_user", "deleted_user", "feature_disabled", "redis_down"} {
		t.Run(name, func(t *testing.T) {
			f := newDesktopFixture(t)
			req := f.tokenRequest(t)
			switch name {
			case "expired":
				f.redis.FastForward(service.DesktopAuthCodeTTL + time.Second)
			case "password_changed":
				f.user.PasswordHash = "changed-hash"
			case "disabled_user":
				f.user.Status = "disabled"
			case "deleted_user":
				now := time.Now()
				f.user.DeletedAt = &now
			case "feature_disabled":
				f.cfg.DesktopAuth.Enabled = false
			case "redis_down":
				f.redis.Close()
			}
			w := f.post("/token", req, false)
			require.GreaterOrEqual(t, w.Code, 400, w.Body.String())
			require.NotContains(t, w.Body.String(), "access_token")
		})
	}
}

func TestDesktopAuthorizationRequiresConsentAndRejectsRedirects(t *testing.T) {
	f := newDesktopFixture(t)
	require.Equal(t, http.StatusUnauthorized, f.post("/authorize", f.request, false).Code)
	require.Equal(t, http.StatusBadRequest, f.post("/authorize", f.request, true).Code)
	for _, change := range []func(*service.DesktopAuthorizeRequest){
		func(r *service.DesktopAuthorizeRequest) { r.RedirectURI = "https://evil.example/callback" },
		func(r *service.DesktopAuthorizeRequest) { r.RedirectURI += "?code=injected" },
		func(r *service.DesktopAuthorizeRequest) { r.RedirectURI = "sub2api://oauth@evil.example/callback" },
		func(r *service.DesktopAuthorizeRequest) { r.ClientID = "other-client" },
		func(r *service.DesktopAuthorizeRequest) { r.CodeChallengeMethod = "plain" },
		func(r *service.DesktopAuthorizeRequest) { r.CodeChallenge = "" },
		func(r *service.DesktopAuthorizeRequest) { r.State = "" },
		func(r *service.DesktopAuthorizeRequest) { r.Scope = "admin" },
	} {
		req := f.request
		change(&req)
		w := f.post("/authorize", struct {
			service.DesktopAuthorizeRequest
			Decision string `json:"decision"`
		}{req, "approve"}, true)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Empty(t, w.Header().Get("Location"))
	}
	require.Empty(t, f.redis.Keys())
	callback, err := url.Parse(f.authorize(t, "deny"))
	require.NoError(t, err)
	require.Equal(t, "access_denied", callback.Query().Get("error"))
	require.Equal(t, f.request.State, callback.Query().Get("state"))
	require.Empty(t, callback.Query().Get("code"))
	require.Empty(t, f.redis.Keys())
}

func TestDesktopTokenSupportsFormEncoding(t *testing.T) {
	f := newDesktopFixture(t)
	token := f.tokenRequest(t)
	form := url.Values{"grant_type": {token.GrantType}, "client_id": {token.ClientID}, "redirect_uri": {token.RedirectURI}, "code": {token.Code}, "code_verifier": {token.CodeVerifier}}
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
