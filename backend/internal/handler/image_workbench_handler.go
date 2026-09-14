package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	imageWorkbenchDefaultModel = "gpt-image-2"
	// JSON generation stays small; multipart edits need room for reference images
	// (OpenAI allows ~20MB per part). Cap total request body at 64MB.
	ImageWorkbenchMaxRequestBody = 64 << 20
	imageWorkbenchMinDim         = 256
	imageWorkbenchMaxDim         = 4096
	imageWorkbenchMaxN           = 10
	imageWorkbenchMaxImages      = 4
	imageWorkbenchMaxImageBytes  = 20 << 20
	imageWorkbenchMaxModelLen    = 64
	// imageWorkbenchModelCacheTTL keeps the upstream model catalog from being
	// re-fetched on every page load.
	imageWorkbenchModelCacheTTL = 5 * time.Minute
	// imageWorkbenchModelsMaxBytes caps the upstream /v1/models response we read.
	imageWorkbenchModelsMaxBytes = 4 << 20
)

// imageWorkbenchFallbackModels mirrors the image model ids chatgpt2api advertises
// by default. It keeps the picker usable when the upstream is unreachable.
var imageWorkbenchFallbackModels = []string{"gpt-image-2", "codex-gpt-image-2"}

// imageWorkbenchPresetSizes mirrors the presets exposed by chatgpt2api's image composer.
var imageWorkbenchPresetSizes = []string{
	"1024x1024", // 1:1
	"1024x1536", // 2:3
	"1536x1024", // 3:2
	"1024x1360", // 3:4
	"1360x1024", // 4:3
	"1088x1920", // 9:16
	"1920x1088", // 16:9
	"2048x2048", // 1:1 2K
	"2560x1440", // 16:9 2K
	"1440x2560", // 9:16 2K
	"3840x2160", // 16:9 4K
	"2160x3840", // 9:16 4K
	"auto",
}

type ImageWorkbenchHandler struct {
	apiKeys         imageWorkbenchAPIKeyProvider
	auth            middleware.APIKeyAuthMiddleware
	async           *AsyncImageHandler
	imageChat       *config.ImageChatConfig
	imageChatClient *http.Client
	upstreams       imageWorkbenchUpstreamResolver
	submit          gin.HandlerFunc
	get             gin.HandlerFunc

	// modelsCache memoizes the upstream image model catalog per upstream so the
	// model picker does not hit chatgpt2api on every page load.
	modelsMu    sync.Mutex
	modelsCache map[string]imageWorkbenchModelsCacheEntry
}

type imageWorkbenchModelsCacheEntry struct {
	at     time.Time
	models []string
}

type imageWorkbenchAPIKeyProvider interface {
	GetOrCreateImageWorkbenchKey(ctx context.Context, userID int64) (*service.APIKey, error)
	// GetImageWorkbenchGroupID exposes the group selection behind
	// GetOrCreateImageWorkbenchKey so the model catalog can target the matching
	// upstream.
	GetImageWorkbenchGroupID(ctx context.Context, userID int64) (int64, error)
}

type imageWorkbenchSubmitRequest struct {
	Prompt     string `json:"prompt"`
	Model      string `json:"model"`
	Size       string `json:"size"`
	Quality    string `json:"quality"`
	Background string `json:"background"`
	N          int    `json:"n"`
}

type imageWorkbenchUpload struct {
	FileName    string
	ContentType string
	Data        []byte
}

func NewImageWorkbenchHandler(
	apiKeys *service.APIKeyService,
	auth middleware.APIKeyAuthMiddleware,
	async *AsyncImageHandler,
) *ImageWorkbenchHandler {
	h := &ImageWorkbenchHandler{apiKeys: apiKeys, auth: auth, async: async}
	h.submit = async.Submit
	h.get = async.Get
	return h
}

// SetImageChatConfig wires the server-side chatgpt2api destination after construction.
func (h *ImageWorkbenchHandler) SetImageChatConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	h.imageChat = &cfg.ImageChat
	timeout := time.Duration(cfg.ImageChat.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	h.imageChatClient = &http.Client{Timeout: timeout}
	// A reconfigured upstream invalidates whatever catalog was cached before.
	h.modelsMu.Lock()
	h.modelsCache = nil
	h.modelsMu.Unlock()
}

// SetUpstreamResolver wires the image-group account lookup used to discover the
// chatgpt2api instance that owns the image model catalog.
func (h *ImageWorkbenchHandler) SetUpstreamResolver(resolver imageWorkbenchUpstreamResolver) {
	if h == nil {
		return
	}
	h.upstreams = resolver
	h.modelsMu.Lock()
	h.modelsCache = nil
	h.modelsMu.Unlock()
}

func (h *ImageWorkbenchHandler) Config(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h == nil || h.async == nil || !h.async.enabled() {
		response.Error(c, http.StatusServiceUnavailable, "Online image generation is not configured")
		return
	}
	if _, err := h.apiKeys.GetOrCreateImageWorkbenchKey(c.Request.Context(), subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"ready":                           true,
		"models":                          imageWorkbenchModelCatalog(),
		"sizes":                           append([]string(nil), imageWorkbenchPresetSizes...),
		"qualities":                       []string{"auto", "low", "medium", "high"},
		"max_n":                           imageWorkbenchMaxN,
		"max_images":                      imageWorkbenchMaxImages,
		"supports_edit":                   true,
		"supports_transparent_background": true,
	})
}

// Models lists the image models the image group's chatgpt2api upstream currently
// advertises. chatgpt2api exposes the authoritative list through /api/model-catalog
// and, on older builds, through its OpenAI-compatible /v1/models endpoint; we keep
// only ids that carry an "image" marker for the latter. When the upstream is
// unreachable we fall back to the built-in catalog so the page stays usable.
func (h *ImageWorkbenchHandler) Models(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	upstream := h.resolveUpstream(c.Request.Context(), subject.UserID)
	models, source := h.availableImageModels(c.Request.Context(), upstream)
	response.Success(c, gin.H{
		"models": models,
		"source": source,
	})
}

func (h *ImageWorkbenchHandler) availableImageModels(ctx context.Context, upstream *imageWorkbenchUpstream) ([]string, string) {
	if models := h.cachedUpstreamImageModels(ctx, upstream); len(models) > 0 {
		return models, "upstream"
	}
	return imageWorkbenchModelCatalog(), "fallback"
}

// resolveUpstream prefers the account backing the caller's image group so the
// picker matches what generation actually reaches, then falls back to an
// explicitly configured image_chat destination.
func (h *ImageWorkbenchHandler) resolveUpstream(ctx context.Context, userID int64) *imageWorkbenchUpstream {
	if h == nil {
		return nil
	}
	if h.upstreams != nil && h.apiKeys != nil {
		if groupID, err := h.apiKeys.GetImageWorkbenchGroupID(ctx, userID); err == nil {
			if upstream, rerr := h.upstreams.ResolveImageWorkbenchUpstream(ctx, groupID); rerr == nil && upstream != nil && upstream.BaseURL != "" {
				return upstream
			}
		}
	}
	if h.imageChat != nil && h.imageChat.Enabled {
		if base := strings.TrimRight(strings.TrimSpace(h.imageChat.BaseURL), "/"); base != "" {
			return &imageWorkbenchUpstream{BaseURL: base, APIKey: strings.TrimSpace(h.imageChat.APIKey)}
		}
	}
	return nil
}

// cachedUpstreamImageModels returns the upstream catalog, reusing a short-lived
// cache per upstream so a busy page does not translate into a request storm on
// chatgpt2api.
func (h *ImageWorkbenchHandler) cachedUpstreamImageModels(ctx context.Context, upstream *imageWorkbenchUpstream) []string {
	if h == nil || upstream == nil || upstream.BaseURL == "" {
		return nil
	}
	cacheKey := upstream.BaseURL
	h.modelsMu.Lock()
	if entry, found := h.modelsCache[cacheKey]; found && len(entry.models) > 0 && time.Since(entry.at) < imageWorkbenchModelCacheTTL {
		cached := append([]string(nil), entry.models...)
		h.modelsMu.Unlock()
		return cached
	}
	h.modelsMu.Unlock()

	models := h.fetchUpstreamImageModels(ctx, upstream)
	if len(models) == 0 {
		return nil
	}
	h.modelsMu.Lock()
	if h.modelsCache == nil {
		h.modelsCache = make(map[string]imageWorkbenchModelsCacheEntry)
	}
	h.modelsCache[cacheKey] = imageWorkbenchModelsCacheEntry{at: time.Now(), models: append([]string(nil), models...)}
	h.modelsMu.Unlock()
	return models
}

func (h *ImageWorkbenchHandler) fetchUpstreamImageModels(ctx context.Context, upstream *imageWorkbenchUpstream) []string {
	if h == nil || upstream == nil || upstream.BaseURL == "" {
		return nil
	}
	base := strings.TrimRight(strings.TrimSpace(upstream.BaseURL), "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil
	}
	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	if _, hasDeadline := requestCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(requestCtx, 15*time.Second)
		defer cancel()
	}

	// chatgpt2api advertises the authoritative image model list (including
	// account-derived ids such as "plus-codex-gpt-image-2") through its model
	// catalog endpoint. Older builds only ship the OpenAI-style list, so we fall
	// back to filtering that one by the "image" marker.
	if models := h.fetchUpstreamModelCatalog(requestCtx, base, upstream.APIKey); len(models) > 0 {
		return models
	}
	return h.fetchUpstreamOpenAIModels(requestCtx, base, upstream.APIKey)
}

// fetchUpstreamModelCatalog reads image_models from chatgpt2api's
// /api/model-catalog endpoint.
func (h *ImageWorkbenchHandler) fetchUpstreamModelCatalog(ctx context.Context, base, apiKey string) []string {
	body, ok := h.getUpstreamJSON(ctx, base+"/api/model-catalog", apiKey)
	if !ok {
		return nil
	}
	var payload struct {
		ImageModels []string `json:"image_models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	return dedupeImageWorkbenchModels(payload.ImageModels, nil)
}

// fetchUpstreamOpenAIModels filters the OpenAI-compatible /v1/models list down to
// the ids that advertise image generation.
func (h *ImageWorkbenchHandler) fetchUpstreamOpenAIModels(ctx context.Context, base, apiKey string) []string {
	body, ok := h.getUpstreamJSON(ctx, base+"/v1/models", apiKey)
	if !ok {
		return nil
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	ids := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		ids = append(ids, item.ID)
	}
	models := dedupeImageWorkbenchModels(ids, func(id string) bool {
		return strings.Contains(strings.ToLower(id), "image")
	})
	sort.Strings(models)
	return models
}

// getUpstreamJSON performs an authenticated GET against chatgpt2api and returns
// the raw body. Every failure is signalled as ok=false so callers can fall back.
func (h *ImageWorkbenchHandler) getUpstreamJSON(ctx context.Context, endpoint, apiKey string) ([]byte, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("Accept", "application/json")
	if key := strings.TrimSpace(apiKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := h.imageChatClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, imageWorkbenchModelsMaxBytes))
	if err != nil {
		return nil, false
	}
	return body, true
}

// dedupeImageWorkbenchModels trims ids, drops blanks and duplicates, and applies
// an optional keep filter while preserving the upstream ordering.
func dedupeImageWorkbenchModels(ids []string, keep func(string) bool) []string {
	seen := make(map[string]struct{}, len(ids))
	models := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" || (keep != nil && !keep(id)) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	return models
}

// imageWorkbenchModelCatalog returns a fresh copy of the built-in catalog.
func imageWorkbenchModelCatalog() []string {
	return append([]string(nil), imageWorkbenchFallbackModels...)
}

// normalizeImageWorkbenchModel keeps the forwarded model id to a conservative
// ASCII charset so a caller cannot smuggle anything into the upstream payload.
// An empty value means "use the default".
func normalizeImageWorkbenchModel(value string) (string, bool) {
	model := strings.TrimSpace(value)
	if model == "" {
		return imageWorkbenchDefaultModel, true
	}
	if len(model) > imageWorkbenchMaxModelLen {
		return "", false
	}
	for _, r := range model {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-', r == ':':
		default:
			return "", false
		}
	}
	return model, true
}

func (h *ImageWorkbenchHandler) Submit(c *gin.Context) {
	// With reference images → image edit (multipart → /v1/images/edits/async).
	// Without → text-to-image generation (JSON → /v1/images/generations/async).
	// Mirrors chatgpt2api: files present switches mode from generate to edit.
	if isMultipartImagesContentType(c.GetHeader("Content-Type")) {
		h.submitEdit(c)
		return
	}
	h.submitGenerate(c)
}

func (h *ImageWorkbenchHandler) submitGenerate(c *gin.Context) {
	var input imageWorkbenchSubmitRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid image generation request")
		return
	}
	prompt, size, quality, background, n, ok := normalizeImageWorkbenchControls(c, input.Prompt, input.Size, input.Quality, input.Background, input.N)
	if !ok {
		return
	}
	model, ok := normalizeImageWorkbenchModel(input.Model)
	if !ok {
		response.BadRequest(c, "Unsupported image model")
		return
	}
	requestPayload := gin.H{
		"model":   model,
		"prompt":  prompt,
		"size":    size,
		"quality": quality,
		"n":       n,
	}
	if background != "" {
		requestPayload["background"] = background
	}
	payload, err := json.Marshal(requestPayload)
	if err != nil {
		response.InternalError(c, "Failed to prepare image generation request")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(payload))
	c.Request.ContentLength = int64(len(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(imageTaskPollBaseContextKey, "/api/v1/image-workbench/tasks")
	h.withManagedKey(c, "/v1/images/generations/async", h.submit)
}

func (h *ImageWorkbenchHandler) submitEdit(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(imageWorkbenchMaxImageBytes); err != nil {
		response.BadRequest(c, "Invalid multipart image edit request")
		return
	}
	form := c.Request.MultipartForm
	if form == nil {
		response.BadRequest(c, "Invalid multipart image edit request")
		return
	}

	prompt := firstFormValue(form, "prompt")
	model, ok := normalizeImageWorkbenchModel(firstFormValue(form, "model"))
	if !ok {
		response.BadRequest(c, "Unsupported image model")
		return
	}
	size := firstFormValue(form, "size")
	quality := firstFormValue(form, "quality")
	background := firstFormValue(form, "background")
	n := 1
	if rawN := firstFormValue(form, "n"); rawN != "" {
		parsed, err := strconv.Atoi(rawN)
		if err != nil {
			response.BadRequest(c, "Unsupported image count")
			return
		}
		n = parsed
	}
	prompt, size, quality, background, n, ok = normalizeImageWorkbenchControls(c, prompt, size, quality, background, n)
	if !ok {
		return
	}

	uploads, err := collectImageWorkbenchUploads(form)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(uploads) == 0 {
		response.BadRequest(c, "At least one reference image is required for edit")
		return
	}
	if len(uploads) > imageWorkbenchMaxImages {
		response.BadRequest(c, "Too many reference images")
		return
	}

	body, contentType, err := buildImageWorkbenchEditMultipart(prompt, model, size, quality, background, n, uploads)
	if err != nil {
		response.InternalError(c, "Failed to prepare image edit request")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Type", contentType)
	c.Set(imageTaskPollBaseContextKey, "/api/v1/image-workbench/tasks")
	h.withManagedKey(c, "/v1/images/edits/async", h.submit)
}

func (h *ImageWorkbenchHandler) Get(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" || strings.Contains(taskID, "/") {
		response.BadRequest(c, "Invalid task ID")
		return
	}
	h.withManagedKey(c, "/v1/images/tasks/"+taskID, h.get)
}

func (h *ImageWorkbenchHandler) withManagedKey(c *gin.Context, gatewayPath string, next gin.HandlerFunc) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h == nil || h.apiKeys == nil || h.async == nil || next == nil {
		response.Error(c, http.StatusServiceUnavailable, "Online image generation is unavailable")
		return
	}
	key, err := h.apiKeys.GetOrCreateImageWorkbenchKey(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	c.Request.URL.Path = gatewayPath
	c.Request.Header.Set("Authorization", "Bearer "+key.Key)
	gin.HandlerFunc(h.auth)(c)
	if c.IsAborted() {
		return
	}
	next(c)
}

func normalizeImageWorkbenchControls(c *gin.Context, prompt, size, quality, background string, n int) (string, string, string, string, int, bool) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		response.BadRequest(c, "Prompt is required")
		return "", "", "", "", 0, false
	}
	if len(prompt) > 32000 {
		response.BadRequest(c, "Prompt is too long")
		return "", "", "", "", 0, false
	}
	size = strings.TrimSpace(size)
	if size == "" {
		size = "1024x1024"
	}
	if !validImageWorkbenchSize(size) {
		response.BadRequest(c, "Unsupported image size")
		return "", "", "", "", 0, false
	}
	quality = strings.TrimSpace(quality)
	if quality == "" {
		quality = "auto"
	}
	if !allowedImageWorkbenchValue(quality, "auto", "low", "medium", "high") {
		response.BadRequest(c, "Unsupported image quality")
		return "", "", "", "", 0, false
	}
	background = strings.ToLower(strings.TrimSpace(background))
	if background != "" && background != "transparent" {
		response.BadRequest(c, "Unsupported image background")
		return "", "", "", "", 0, false
	}
	if n <= 0 {
		n = 1
	}
	if n > imageWorkbenchMaxN {
		response.BadRequest(c, "Unsupported image count")
		return "", "", "", "", 0, false
	}
	return prompt, size, quality, background, n, true
}

func firstFormValue(form *multipart.Form, key string) string {
	if form == nil {
		return ""
	}
	values := form.Value[key]
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func collectImageWorkbenchUploads(form *multipart.Form) ([]imageWorkbenchUpload, error) {
	if form == nil {
		return nil, nil
	}
	var uploads []imageWorkbenchUpload
	for key, headers := range form.File {
		// OpenAI edits accept "image" or "image[]" / "image[0]" style fields.
		if key != "image" && !strings.HasPrefix(key, "image[") {
			continue
		}
		for _, header := range headers {
			if header == nil {
				continue
			}
			if header.Size > imageWorkbenchMaxImageBytes {
				return nil, errImageWorkbench("reference image is too large")
			}
			file, err := header.Open()
			if err != nil {
				return nil, errImageWorkbench("failed to read reference image")
			}
			data, err := io.ReadAll(io.LimitReader(file, imageWorkbenchMaxImageBytes+1))
			_ = file.Close()
			if err != nil {
				return nil, errImageWorkbench("failed to read reference image")
			}
			if len(data) == 0 {
				continue
			}
			if len(data) > imageWorkbenchMaxImageBytes {
				return nil, errImageWorkbench("reference image is too large")
			}
			contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			if !isAllowedImageWorkbenchUploadType(contentType, header.Filename) {
				return nil, errImageWorkbench("reference image must be png, jpeg, webp, or gif")
			}
			name := strings.TrimSpace(header.Filename)
			if name == "" {
				name = "reference.png"
			}
			uploads = append(uploads, imageWorkbenchUpload{
				FileName:    name,
				ContentType: contentType,
				Data:        data,
			})
		}
	}
	return uploads, nil
}

func isAllowedImageWorkbenchUploadType(contentType, fileName string) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch ct {
	case "image/png", "image/jpeg", "image/jpg", "image/webp", "image/gif":
		return true
	}
	// Some browsers send application/octet-stream; fall back to extension.
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return true
	}
	return false
}

func buildImageWorkbenchEditMultipart(prompt, model, size, quality, background string, n int, uploads []imageWorkbenchUpload) ([]byte, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fields := map[string]string{
		"model":   model,
		"prompt":  prompt,
		"size":    size,
		"quality": quality,
		"n":       strconv.Itoa(n),
	}
	if background != "" {
		fields["background"] = background
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			_ = writer.Close()
			return nil, "", err
		}
	}
	for _, upload := range uploads {
		part, err := writer.CreateFormFile("image", upload.FileName)
		if err != nil {
			_ = writer.Close()
			return nil, "", err
		}
		if _, err := part.Write(upload.Data); err != nil {
			_ = writer.Close()
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

type imageWorkbenchError string

func (e imageWorkbenchError) Error() string { return string(e) }

func errImageWorkbench(message string) error {
	return imageWorkbenchError(message)
}

func allowedImageWorkbenchValue(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

// validImageWorkbenchSize accepts "auto", chatgpt2api-style presets, or free-form WxH
// within [256, 4096] — same range the chatgpt2api composer allows users to type.
func validImageWorkbenchSize(size string) bool {
	if size == "auto" {
		return true
	}
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return false
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil {
		return false
	}
	return w >= imageWorkbenchMinDim && w <= imageWorkbenchMaxDim &&
		h >= imageWorkbenchMinDim && h <= imageWorkbenchMaxDim
}
