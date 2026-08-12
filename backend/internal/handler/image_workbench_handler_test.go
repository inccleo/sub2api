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

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type imageWorkbenchKeyProviderStub struct {
	key *service.APIKey
	err error
}

func (s imageWorkbenchKeyProviderStub) GetOrCreateImageWorkbenchKey(context.Context, int64) (*service.APIKey, error) {
	return s.key, s.err
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/image-workbench/tasks", strings.NewReader(`{"prompt":"  neon city  ","size":"1536x1024","quality":"high","model":"other","n":8}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer jwt-user-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "/v1/images/generations/async", gotPath)
	require.Equal(t, "Bearer sk-managed", gotAuthorization)
	require.Equal(t, imageWorkbenchModel, gotPayload["model"])
	require.Equal(t, "neon city", gotPayload["prompt"])
	require.Equal(t, "1536x1024", gotPayload["size"])
	require.Equal(t, "high", gotPayload["quality"])
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/image-workbench/tasks", strings.NewReader(`{"prompt":"sky","size":"1920x1088","quality":"medium"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "1920x1088", gotPayload["size"])
	require.Equal(t, float64(1), gotPayload["n"])
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

	// n above max is rejected.
	req = httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"prompt":"cat","size":"1024x1024","quality":"auto","n":99}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Unsupported image count")
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
	require.Equal(t, imageWorkbenchModel, fields["model"])
	require.Equal(t, "make it blue", fields["prompt"])
	require.Equal(t, "1024x1024", fields["size"])
	require.Equal(t, "high", fields["quality"])
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
