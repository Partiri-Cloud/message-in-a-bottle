package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/partiri-cloud/message-in-a-bottle/internal/handler/dto"
	"github.com/redis/go-redis/v9"
)

const announcementKeyPrefix = "miab:announcement:"

type AnnouncementHandler struct {
	rdb *redis.Client
}

func NewAnnouncementHandler(rdb *redis.Client) *AnnouncementHandler {
	return &AnnouncementHandler{rdb: rdb}
}

func (h *AnnouncementHandler) Create(c *gin.Context) {
	var req dto.CreateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	now := time.Now().UTC()
	if req.ExpiresAt.Before(now) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "expiresAt must be in the future"}})
		return
	}

	a := dto.Announcement{
		ID:        uuid.NewString(),
		Severity:  req.Severity,
		Title:     req.Title,
		Message:   req.Message,
		StartsAt:  req.StartsAt,
		ExpiresAt: req.ExpiresAt,
		CreatedAt: now,
	}

	data, err := json.Marshal(a)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	ttl := time.Until(req.ExpiresAt)
	if err := h.rdb.Set(context.Background(), announcementKeyPrefix+a.ID, data, ttl).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": a})
}

func (h *AnnouncementHandler) List(c *gin.Context) {
	announcements, err := h.scanAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	sort.Slice(announcements, func(i, j int) bool {
		return announcements[i].CreatedAt.After(announcements[j].CreatedAt)
	})

	c.JSON(http.StatusOK, gin.H{"data": announcements})
}

func (h *AnnouncementHandler) Active(c *gin.Context) {
	all, err := h.scanAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	now := time.Now().UTC()
	var active []dto.Announcement
	for _, a := range all {
		if a.StartsAt != nil && a.StartsAt.After(now) {
			continue
		}
		active = append(active, a)
	}

	sort.Slice(active, func(i, j int) bool {
		return active[i].CreatedAt.After(active[j].CreatedAt)
	})

	if active == nil {
		active = []dto.Announcement{}
	}

	c.JSON(http.StatusOK, gin.H{"data": active})
}

func (h *AnnouncementHandler) Update(c *gin.Context) {
	id := c.Param("id")
	key := announcementKeyPrefix + id

	raw, err := h.rdb.Get(c.Request.Context(), key).Bytes()
	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "announcement not found"}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	var a dto.Announcement
	if err := json.Unmarshal(raw, &a); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	var req dto.UpdateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	if req.Severity != nil {
		a.Severity = *req.Severity
	}
	if req.Title != nil {
		a.Title = *req.Title
	}
	if req.Message != nil {
		a.Message = *req.Message
	}
	if req.StartsAt != nil {
		a.StartsAt = req.StartsAt
	}
	if req.ExpiresAt != nil {
		a.ExpiresAt = *req.ExpiresAt
	}

	now := time.Now().UTC()
	if a.ExpiresAt.Before(now) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "expiresAt must be in the future"}})
		return
	}

	data, err := json.Marshal(a)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	ttl := time.Until(a.ExpiresAt)
	if err := h.rdb.Set(c.Request.Context(), key, data, ttl).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": a})
}

func (h *AnnouncementHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	key := announcementKeyPrefix + id

	n, err := h.rdb.Del(c.Request.Context(), key).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "announcement not found"}})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *AnnouncementHandler) scanAll(ctx context.Context) ([]dto.Announcement, error) {
	var keys []string
	iter := h.rdb.Scan(ctx, 0, announcementKeyPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return []dto.Announcement{}, nil
	}

	vals, err := h.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	var announcements []dto.Announcement
	for _, v := range vals {
		if v == nil {
			continue
		}
		var a dto.Announcement
		if err := json.Unmarshal([]byte(v.(string)), &a); err != nil {
			continue
		}
		announcements = append(announcements, a)
	}

	return announcements, nil
}
