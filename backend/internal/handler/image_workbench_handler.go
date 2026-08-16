package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	imageWorkbenchModel = "gpt-image-2"
	// JSON generation stays small; multipart edits need room for reference images
	// (OpenAI allows ~20MB per part). Cap total request body at 64MB.
	ImageWorkbenchMaxRequestBody = 64 << 20
	imageWorkbenchMinDim         = 256
	imageWorkbenchMaxDim         = 4096
	imageWorkbenchMaxN           = 10
	imageWorkbenchMaxImages      = 4
	imageWorkbenchMaxImageBytes  = 20 << 20
)

// imageWorkbenchPresetSizes mirrors chatgpt2api's 1K aspect presets + official OpenAI sizes.
// 2K/4K codex-only presets are intentionally omitted (workbench model is gpt-image-2).
var imageWorkbenchPresetSizes = []string{
	"1024x1024", // 1:1
	"1024x1536", // 2:3
	"1536x1024", // 3:2
	"1024x1365", // 3:4
	"1365x1024", // 4:3
	"1088x1920", // 9:16
	"1920x1088", // 16:9
	"auto",
}

type ImageWorkbenchHandler struct {
	apiKeys imageWorkbenchAPIKeyProvider
	auth    middleware.APIKeyAuthMiddleware
	async   *AsyncImageHandler
	submit  gin.HandlerFunc
	get     gin.HandlerFunc
}

type imageWorkbenchAPIKeyProvider interface {
	GetOrCreateImageWorkbenchKey(ctx context.Context, userID int64) (*service.APIKey, error)
}

type imageWorkbenchSubmitRequest struct {
	Prompt  string `json:"prompt"`
	Size    string `json:"size"`
	Quality string `json:"quality"`
	N       int    `json:"n"`
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
		"ready":         true,
		"models":        []string{imageWorkbenchModel},
		"sizes":         append([]string(nil), imageWorkbenchPresetSizes...),
		"qualities":     []string{"auto", "low", "medium", "high"},
		"max_n":         imageWorkbenchMaxN,
		"max_images":    imageWorkbenchMaxImages,
		"supports_edit": true,
	})
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
	prompt, size, quality, n, ok := normalizeImageWorkbenchControls(c, input.Prompt, input.Size, input.Quality, input.N)
	if !ok {
		return
	}
	payload, err := json.Marshal(gin.H{
		"model":   imageWorkbenchModel,
		"prompt":  prompt,
		"size":    size,
		"quality": quality,
		"n":       n,
	})
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
	size := firstFormValue(form, "size")
	quality := firstFormValue(form, "quality")
	n := 1
	if rawN := firstFormValue(form, "n"); rawN != "" {
		parsed, err := strconv.Atoi(rawN)
		if err != nil {
			response.BadRequest(c, "Unsupported image count")
			return
		}
		n = parsed
	}
	prompt, size, quality, n, ok := normalizeImageWorkbenchControls(c, prompt, size, quality, n)
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

	body, contentType, err := buildImageWorkbenchEditMultipart(prompt, size, quality, n, uploads)
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

func normalizeImageWorkbenchControls(c *gin.Context, prompt, size, quality string, n int) (string, string, string, int, bool) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		response.BadRequest(c, "Prompt is required")
		return "", "", "", 0, false
	}
	if len(prompt) > 32000 {
		response.BadRequest(c, "Prompt is too long")
		return "", "", "", 0, false
	}
	size = strings.TrimSpace(size)
	if size == "" {
		size = "1024x1024"
	}
	if !validImageWorkbenchSize(size) {
		response.BadRequest(c, "Unsupported image size")
		return "", "", "", 0, false
	}
	quality = strings.TrimSpace(quality)
	if quality == "" {
		quality = "auto"
	}
	if !allowedImageWorkbenchValue(quality, "auto", "low", "medium", "high") {
		response.BadRequest(c, "Unsupported image quality")
		return "", "", "", 0, false
	}
	if n <= 0 {
		n = 1
	}
	if n > imageWorkbenchMaxN {
		response.BadRequest(c, "Unsupported image count")
		return "", "", "", 0, false
	}
	return prompt, size, quality, n, true
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

func buildImageWorkbenchEditMultipart(prompt, size, quality string, n int, uploads []imageWorkbenchUpload) ([]byte, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fields := map[string]string{
		"model":   imageWorkbenchModel,
		"prompt":  prompt,
		"size":    size,
		"quality": quality,
		"n":       strconv.Itoa(n),
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
