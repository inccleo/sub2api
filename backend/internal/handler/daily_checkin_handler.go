package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type DailyCheckinHandler struct {
	service *service.DailyCheckinService
}

func NewDailyCheckinHandler(dailyCheckinService *service.DailyCheckinService) *DailyCheckinHandler {
	return &DailyCheckinHandler{service: dailyCheckinService}
}

// GetStatus returns the current user's daily check-in state.
// GET /api/v1/checkin/status
func (h *DailyCheckinHandler) GetStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	status, err := h.service.GetStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// GetHistory returns the current user's recent check-in records.
// GET /api/v1/checkin/history
func (h *DailyCheckinHandler) GetHistory(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	limit := 30
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "Invalid limit")
			return
		}
		limit = parsed
	}

	history, err := h.service.GetHistory(c.Request.Context(), subject.UserID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, history)
}

// Checkin credits today's reward to the current user's balance.
// POST /api/v1/checkin
func (h *DailyCheckinHandler) Checkin(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.service.Checkin(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// AdminList returns paginated check-in records for administrators.
// GET /api/v1/admin/checkins
func (h *DailyCheckinHandler) AdminList(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.DailyCheckinListFilter{
		Page:     page,
		PageSize: pageSize,
		Search:   strings.TrimSpace(c.Query("search")),
	}

	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		userID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || userID <= 0 {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		filter.UserID = &userID
	}

	loc := timezone.Location()
	if loc == nil {
		loc = time.Local
	}
	if raw := strings.TrimSpace(c.Query("start_date")); raw != "" {
		startDate, err := time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			response.BadRequest(c, "Invalid start_date, expected YYYY-MM-DD")
			return
		}
		filter.StartDate = &startDate
	}
	if raw := strings.TrimSpace(c.Query("end_date")); raw != "" {
		endDate, err := time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			response.BadRequest(c, "Invalid end_date, expected YYYY-MM-DD")
			return
		}
		filter.EndDate = &endDate
	}

	items, total, err := h.service.AdminList(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

// AdminStats returns aggregate check-in metrics for administrators.
// GET /api/v1/admin/checkins/stats
func (h *DailyCheckinHandler) AdminStats(c *gin.Context) {
	stats, err := h.service.AdminStats(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}
