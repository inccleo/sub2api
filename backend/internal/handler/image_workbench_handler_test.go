package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type imageWorkbenchKeyProviderStub struct {
	key     *service.APIKey
	err     error
	groupID int64
}

func (s imageWorkbenchKeyProviderStub) GetOrCreateImageWorkbenchKey(context.Context, int64) (*service.APIKey, error) {
	return s.key, s.err
}

func (s imageWorkbenchKeyProviderStub) GetImageWorkbenchGroupID(context.Context, int64) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.groupID, nil
}

// imageWorkbenchUpstreamStub resolves every group to the same chatgpt2api
// endpoint so the model catalog tests stay focused on parsing and caching.
type imageWorkbenchUpstreamStub struct {
	upstream *imageWorkbenchUpstream
	calls    int
}

func (s *imageWorkbenchUpstreamStub) ResolveImageWorkbenchUpstream(context.Context, int64) (*imageWorkbenchUpstream, error) {
	s.calls++
	if s.upstream == nil {
		return nil, service.ErrImageWorkbenchUnavailable
	}
	return s.upstream, nil
}

func TestImageWorkbenchConfigReturnsUnavailableWithoutEligibleGroup(t *testing.T) {
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	h := &ImageWorkbenchHandler{
		apiKeys: imageWorkbenchKeyProviderStub{err: service.ErrImageWorkbenchUnavailable},
		async:   &AsyncImageHandler{tasks: tasks},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	})
	router.GET("/api/v1/image-workbench/config", h.Config)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/image-workbench/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), `"reason":"IMAGE_WORKBENCH_UNAVAILABLE"`)
}

func TestImageWorkbenchSubmitUsesManagedKeyAndForcesPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotPath, gotAuthorization string
	var gotPayload map[string]any
	auth := middleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		gotPath = c.Request.URL.Path
		gotAuthorization = c.GetHeader("Authorization")
	})
	h := &ImageWorkbenchHandler{
		apiKeys: imageWorkbenchKeyProviderStub{key: &service.APIKey{Key: "sk-managed"}},
		auth:    auth,
		async:   &AsyncImageHandler{},
		submit: func(c *gin.Context) {
			body, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &gotPayload))
			c.Status(http.StatusAccepted)
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	})
	router.POST("/api/v1/image-workbench/tasks", h.Submit)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/image-workbench/tasks", strings.NewReader(`{"prompt":"  neon city  ","size":"1536x1024","quality":"high","background":"transparent","model":"codex-gpt-image-2","n":8}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer jwt-user-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "/v1/images/generations/async", gotPath)
	require.Equal(t, "Bearer sk-managed", gotAuthorization)
	// A caller-supplied model is honored; the default only kicks in when omitted.
	require.Equal(t, "codex-gpt-image-2", gotPayload["model"])
	require.Equal(t, "neon city", gotPayload["prompt"])
	require.Equal(t, "1536x1024", gotPayload["size"])
	require.Equal(t, "high", gotPayload["quality"])
	require.Equal(t, "transparent", gotPayload["background"])
	// Client-supplied n is accepted (clamped 1..max); older clients that omit n still get 1.
	require.Equal(t, float64(8), gotPayload["n"])
}

func TestImageWorkbenchSubmitAcceptsChatGPT2APIStyleSizeAndDefaultN(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotPayload map[string]any
	auth := middleware.APIKeyAuthMiddleware(func(c *gin.Context) {})
	h := &ImageWorkbenchHandler{
		apiKeys: imageWorkbenchKeyProviderStub{key: &service.APIKey{Key: "sk-managed"}},
		auth:    auth,
		async:   &AsyncImageHandler{},
		submit: func(c *gin.Context) {
			body, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &gotPayload))
			c.Status(http.StatusAccepted)
		},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	})
	router.POST("/api/v1/image-workbench/tasks", h.Submit)

	for _, size := range []string{
		"1920x1088",
		"2048x2048",
		"2560x1440",
		"1440x2560",
		"3840x2160",
		"2160x3840",
	} {
		t.Run(size, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/image-workbench/tasks", strings.NewReader(`{"prompt":"sky","size":"`+size+`","quality":"medium"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusAccepted, w.Code)
			require.Equal(t, size, gotPayload["size"])
			require.Equal(t, float64(1), gotPayload["n"])
			require.NotContains(t, gotPayload, "background")
		})
	}
}

func TestImageWorkbenchSubmitReturnsJWTWorkbenchPollURL(t *testing.T) {
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	async := &AsyncImageHandler{tasks: tasks}
	async.execute = func(_ string, c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{{"url": "https://example.test/image.png"}}})
	}
	h := &ImageWorkbenchHandler{
		apiKeys: imageWorkbenchKeyProviderStub{key: &service.APIKey{Key: "sk-managed"}},
		auth: middleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			groupID := int64(3)
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 9, UserID: 7, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true}})
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		}),
		async: async,
	}
	h.submit = async.Submit

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7}) })
	router.POST("/api/v1/image-workbench/tasks", h.Submit)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/image-workbench/tasks", strings.NewReader(`{"prompt":"cat","size":"1024x1024","quality":"auto"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.Contains(t, w.Body.String(), `"poll_url":"/api/v1/image-workbench/tasks/imgtask_`)
}

func TestImageWorkbenchSubmitRejectsUnsupportedControls(t *testing.T) {
	h := &ImageWorkbenchHandler{async: &AsyncImageHandler{}}
	router := gin.New()
	router.POST("/tasks", h.Submit)

	// Dimensions above the workbench cap are rejected.
	req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"prompt":"cat","size":"8192x8192","quality":"auto"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Unsupported image size")

	// Quality outside OpenAI's set is rejected.
	req = httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"prompt":"cat","size":"1024x1024","quality":"ultra"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Unsupported image quality")

	// Only the transparent opt-in is accepted by the hosted workbench.
	req = httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"prompt":"cat","size":"1024x1024","quality":"auto","background":"opaque"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Unsupported image background")

	// n above max is rejected.
	req = httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"prompt":"cat","size":"1024x1024","quality":"auto","n":99}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Unsupported image count")

	// Model ids outside the conservative charset are rejected.
	req = httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"prompt":"cat","size":"1024x1024","quality":"auto","model":"gpt-image-2/../../etc"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Unsupported image model")
}

func TestImageWorkbenchModelsFallsBackWhenUpstreamUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ImageWorkbenchHandler{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	})
	router.GET("/models", h.Models)

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"source":"fallback"`)
	require.Contains(t, w.Body.String(), "gpt-image-2")
}

func TestImageWorkbenchModelsFiltersUpstreamCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer downstream-secret", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/model-catalog":
			// Older chatgpt2api builds do not expose the catalog, so this must
			// degrade to the /v1/models filter instead of failing the request.
			w.WriteHeader(http.StatusNotFound)
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5"},{"id":"gpt-image-2"},{"id":"codex-gpt-image-2"},{"id":"gpt-image-2"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	h := NewImageWorkbenchHandler(nil, nil, nil)
	h.SetImageChatConfig(&config.Config{ImageChat: config.ImageChatConfig{Enabled: true, BaseURL: upstream.URL, APIKey: "downstream-secret"}})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	})
	router.GET("/models", h.Models)

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"source":"upstream"`)
	require.NotContains(t, w.Body.String(), "gpt-5")
	var payload struct {
		Data struct {
			Models []string `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, []string{"codex-gpt-image-2", "gpt-image-2"}, payload.Data.Models)
}

func TestImageWorkbenchModelsPrefersUpstreamModelCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var catalogRequests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer downstream-secret", r.Header.Get("Authorization"))
		require.Equal(t, "/api/model-catalog", r.URL.Path)
		catalogRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"model_catalog","chat_models":["gpt-5.6"],"image_models":["gpt-image-2","gpt-image-2.5","gpt-image-2.5-flare","gpt-image-2.5-sunburst","codex-gpt-image-2","plus-codex-gpt-image-2","pro-codex-gpt-image-2"]}`))
	}))
	defer upstream.Close()

	h := NewImageWorkbenchHandler(nil, nil, nil)
	h.SetImageChatConfig(&config.Config{ImageChat: config.ImageChatConfig{Enabled: true, BaseURL: upstream.URL, APIKey: "downstream-secret"}})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	})
	router.GET("/models", h.Models)

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, catalogRequests)
	var payload struct {
		Data struct {
			Models []string `json:"models"`
			Source string   `json:"source"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "upstream", payload.Data.Source)
	require.Equal(t, []string{
		"gpt-image-2",
		"gpt-image-2.5",
		"gpt-image-2.5-flare",
		"gpt-image-2.5-sunburst",
		"codex-gpt-image-2",
		"plus-codex-gpt-image-2",
		"pro-codex-gpt-image-2",
	}, payload.Data.Models)
}

func TestImageWorkbenchModelsUsesImageGroupUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var requested []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Path)
		require.Equal(t, "Bearer group-account-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"model_catalog","image_models":["gpt-image-2","gpt-image-2.5","codex-gpt-image-2"]}`))
	}))
	defer upstream.Close()

	resolver := &imageWorkbenchUpstreamStub{upstream: &imageWorkbenchUpstream{BaseURL: upstream.URL, APIKey: "group-account-key"}}
	h := &ImageWorkbenchHandler{apiKeys: imageWorkbenchKeyProviderStub{groupID: 2}, upstreams: resolver}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	})
	router.GET("/models", h.Models)

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, resolver.calls)
	require.Equal(t, []string{"/api/model-catalog"}, requested)
	var payload struct {
		Data struct {
			Models []string `json:"models"`
			Source string   `json:"source"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "upstream", payload.Data.Source)
	require.Equal(t, []string{"gpt-image-2", "gpt-image-2.5", "codex-gpt-image-2"}, payload.Data.Models)
}

func TestImageWorkbenchSubmitEditRoutesToEditsAsync(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotPath, gotContentType string
	var gotBody []byte
	auth := middleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		gotPath = c.Request.URL.Path
		gotContentType = c.GetHeader("Content-Type")
	})
	h := &ImageWorkbenchHandler{
		apiKeys: imageWorkbenchKeyProviderStub{key: &service.APIKey{Key: "sk-managed"}},
		auth:    auth,
		async:   &AsyncImageHandler{},
		submit: func(c *gin.Context) {
			body, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			gotBody = body
			c.Status(http.StatusAccepted)
		},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	})
	router.POST("/api/v1/image-workbench/tasks", h.Submit)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("prompt", " make it blue "))
	require.NoError(t, writer.WriteField("size", "1024x1024"))
	require.NoError(t, writer.WriteField("quality", "high"))
	require.NoError(t, writer.WriteField("background", "transparent"))
	require.NoError(t, writer.WriteField("n", "2"))
	part, err := writer.CreateFormFile("image", "ref.png")
	require.NoError(t, err)
	// Minimal valid-looking PNG header bytes (content is not decoded here).
	_, err = part.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x01})
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/image-workbench/tasks", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "/v1/images/edits/async", gotPath)
	require.Contains(t, gotContentType, "multipart/form-data")

	_, params, err := mime.ParseMediaType(gotContentType)
	require.NoError(t, err)
	reader := multipart.NewReader(bytes.NewReader(gotBody), params["boundary"])
	fields := map[string]string{}
	var fileCount int
	for {
		p, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(p)
		require.NoError(t, err)
		if p.FileName() != "" {
			fileCount++
			require.Equal(t, "image", p.FormName())
			require.NotEmpty(t, data)
		} else {
			fields[p.FormName()] = string(data)
		}
		_ = p.Close()
	}
	require.Equal(t, 1, fileCount)
	require.Equal(t, imageWorkbenchDefaultModel, fields["model"])
	require.Equal(t, "make it blue", fields["prompt"])
	require.Equal(t, "1024x1024", fields["size"])
	require.Equal(t, "high", fields["quality"])
	require.Equal(t, "transparent", fields["background"])
	require.Equal(t, "2", fields["n"])
}

func TestImageWorkbenchSubmitEditRequiresImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ImageWorkbenchHandler{
		apiKeys: imageWorkbenchKeyProviderStub{key: &service.APIKey{Key: "sk-managed"}},
		auth:    middleware.APIKeyAuthMiddleware(func(c *gin.Context) {}),
		async:   &AsyncImageHandler{},
		submit:  func(c *gin.Context) { c.Status(http.StatusAccepted) },
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	})
	router.POST("/tasks", h.Submit)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("prompt", "edit me"))
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/tasks", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "reference image")
}
