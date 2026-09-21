//go:build unit

package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func typeSafeTestAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Name:        "jev",
		Platform:    PlatformTypeSafe,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "ts-test-key",
			"base_url": "https://api.typesafe.ai",
		},
	}
}

func typeSafeAccountTestService(account *Account, resp *http.Response) (*AccountTestService, *httpUpstreamRecorder) {
	upstream := &httpUpstreamRecorder{resp: resp}
	svc := &AccountTestService{
		accountRepo: &mockAccountRepoForGemini{
			accountsByID: map[int64]*Account{account.ID: account},
		},
		httpUpstream: upstream,
		cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
			Enabled: false,
		}}},
	}
	return svc, upstream
}

func TestAccountTestService_TypeSafeUsesSystemOne(t *testing.T) {
	account := typeSafeTestAccount(501)
	svc, upstream := typeSafeAccountTestService(account, &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"model":"jev-1.13.0","answers":{"connectivity":{"type":"noul","noul":0.02}},"usage":{"input_tokens":12,"output_tokens":3}}`,
		)),
	})
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "jev-latest", "hi", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.typesafe.ai/v1/systemone", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer ts-test-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "jev-latest", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "hi", gjson.GetBytes(upstream.lastBody, "state").String())
	require.Equal(t, "noul", gjson.GetBytes(upstream.lastBody, "questions.connectivity.type").String())
	require.NotContains(t, upstream.requests[0].URL.Path, "/messages")
	require.NotContains(t, upstream.requests[0].URL.Path, "/chat/completions")
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
	require.Contains(t, recorder.Body.String(), "jev-1.13.0")
	require.Contains(t, recorder.Body.String(), "正在通过 /v1/systemone 测试连接")
}

func TestAccountTestService_TypeSafeDefaultsEmptyModelAndPrompt(t *testing.T) {
	account := typeSafeTestAccount(502)
	account.Credentials = map[string]any{"api_key": "ts-test-key"}
	svc, upstream := typeSafeAccountTestService(account, &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"model":"jev-latest","answers":{"connectivity":{"type":"noul","noul":0.0}},"usage":{"input_tokens":1,"output_tokens":1}}`)),
	})
	c, _ := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Equal(t, "https://api.typesafe.ai/v1/systemone", upstream.requests[0].URL.String())
	require.Equal(t, "jev-latest", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "hi", gjson.GetBytes(upstream.lastBody, "state").String())
}

func TestAccountTestService_TypeSafeRejectsUnsupportedModelFallback(t *testing.T) {
	account := typeSafeTestAccount(503)
	svc, upstream := typeSafeAccountTestService(account, &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"model":"jev-preview","answers":{"connectivity":{"type":"noul","noul":0.1}},"usage":{"input_tokens":1,"output_tokens":1}}`)),
	})
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "gpt-4o", "probe", AccountTestModeDefault)

	require.NoError(t, err)
	require.Equal(t, "jev-latest", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "probe", gjson.GetBytes(upstream.lastBody, "state").String())
	require.Contains(t, recorder.Body.String(), `"model":"jev-latest"`)
}

func TestAccountTestService_TypeSafeSurfacesSystemOne404(t *testing.T) {
	account := typeSafeTestAccount(504)
	svc, upstream := typeSafeAccountTestService(account, &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"detail":"Not Found"}`)),
	})
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "jev-latest", "hi", AccountTestModeDefault)

	require.Error(t, err)
	require.Equal(t, "https://api.typesafe.ai/v1/systemone", upstream.requests[0].URL.String())
	require.Contains(t, err.Error(), "TypeSafe System One (/v1/systemone) returned 404")
	require.Contains(t, recorder.Body.String(), `Not Found`)
}

func TestAccountTestService_TypeSafeMissingAPIKey(t *testing.T) {
	account := typeSafeTestAccount(505)
	account.Credentials = map[string]any{"base_url": "https://api.typesafe.ai"}
	svc, upstream := typeSafeAccountTestService(account, &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))})
	c, _ := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "jev-latest", "hi", AccountTestModeDefault)

	require.Error(t, err)
	require.Contains(t, err.Error(), "No TypeSafe API key available")
	require.Empty(t, upstream.requests)
}
