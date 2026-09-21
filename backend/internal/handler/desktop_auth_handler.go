package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DesktopAuthHandler struct{ svc *service.DesktopAuthService }

func NewDesktopAuthHandler(svc *service.DesktopAuthService) *DesktopAuthHandler {
	return &DesktopAuthHandler{svc: svc}
}

func desktopAuthHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
}

// Details validates the complete request before the browser displays a consent button.
func (h *DesktopAuthHandler) Details(c *gin.Context) {
	desktopAuthHeaders(c)
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req service.DesktopAuthorizeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid authorization request")
		return
	}
	user, err := h.svc.Describe(c.Request.Context(), subject.UserID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"client_name": "Sub2API Desktop", "redirect_uri": service.DesktopRedirectURI, "scope": "account", "email": user.Email, "is_admin": user.IsAdmin()})
}

func (h *DesktopAuthHandler) Authorize(c *gin.Context) {
	desktopAuthHeaders(c)
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		service.DesktopAuthorizeRequest
		Decision string `json:"decision"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Decision != "approve" && req.Decision != "deny") {
		response.BadRequest(c, "Expected an explicit approve or deny decision")
		return
	}
	callback, err := h.svc.Authorize(c.Request.Context(), subject.UserID, req.DesktopAuthorizeRequest, req.Decision == "approve")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"redirect_uri": callback})
}

func (h *DesktopAuthHandler) Token(c *gin.Context) {
	desktopAuthHeaders(c)
	var req service.DesktopTokenRequest
	if err := c.ShouldBind(&req); err != nil {
		response.BadRequest(c, "Invalid token request")
		return
	}
	pair, user, err := h.svc.Exchange(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, AuthResponse{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresIn: pair.ExpiresIn, TokenType: "Bearer", User: dto.UserFromService(user)})
}
