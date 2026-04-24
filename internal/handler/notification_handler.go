package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-bottle/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-bottle/internal/middleware"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type NotificationHandler struct {
	notifRepo    *repository.NotificationRepository
	activityRepo *repository.ActivityRepository
	subRepo      *repository.SubscriberRepository
}

func NewNotificationHandler(notifRepo *repository.NotificationRepository, activityRepo *repository.ActivityRepository, subRepo *repository.SubscriberRepository) *NotificationHandler {
	return &NotificationHandler{notifRepo: notifRepo, activityRepo: activityRepo, subRepo: subRepo}
}

func (h *NotificationHandler) List(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	page, limit := dto.ParsePagination(c)

	notifs, total, err := h.notifRepo.FindMany(c.Request.Context(), envID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data: notifs,
		Meta: &dto.Meta{Page: page, Limit: limit, Total: total},
	})
}

func (h *NotificationHandler) Get(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid notification ID"}})
		return
	}

	notif, err := h.notifRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "notification not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": notif})
}

func (h *NotificationHandler) Feed(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	subscriberID := c.Param("subscriberId")
	page, limit := dto.ParsePagination(c)

	sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, subscriberID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "subscriber not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	filter := repository.FeedFilter{}
	if v := c.Query("read"); v == "true" {
		t := true
		filter.Read = &t
	} else if v == "false" {
		f := false
		filter.Read = &f
	}
	if v := c.Query("seen"); v == "true" {
		t := true
		filter.Seen = &t
	} else if v == "false" {
		f := false
		filter.Seen = &f
	}

	notifs, total, err := h.notifRepo.FindFeed(c.Request.Context(), envID, sub.ID, filter, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data: notifs,
		Meta: &dto.Meta{Page: page, Limit: limit, Total: total},
	})
}

func (h *NotificationHandler) MarkSeen(c *gin.Context) {
	notifID, err := bson.ObjectIDFromHex(c.Param("notifId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid notification ID"}})
		return
	}

	if err := h.notifRepo.MarkSeen(c.Request.Context(), notifID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	notifID, err := bson.ObjectIDFromHex(c.Param("notifId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid notification ID"}})
		return
	}

	if err := h.notifRepo.MarkRead(c.Request.Context(), notifID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *NotificationHandler) Archive(c *gin.Context) {
	notifID, err := bson.ObjectIDFromHex(c.Param("notifId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid notification ID"}})
		return
	}

	if err := h.notifRepo.MarkArchived(c.Request.Context(), notifID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *NotificationHandler) BulkAction(c *gin.Context) {
	var req dto.BulkActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	var ids []bson.ObjectID
	for _, idStr := range req.NotificationIDs {
		id, err := bson.ObjectIDFromHex(idStr)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}

	var bulkErr error
	switch req.Action {
	case "read":
		bulkErr = h.notifRepo.BulkMarkRead(c.Request.Context(), ids)
	case "seen":
		bulkErr = h.notifRepo.BulkMarkSeen(c.Request.Context(), ids)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "action must be 'read' or 'seen'"}})
		return
	}

	if bulkErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *NotificationHandler) Activity(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	page, limit := dto.ParsePagination(c)

	logs, total, err := h.activityRepo.FindMany(c.Request.Context(), envID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data: logs,
		Meta: &dto.Meta{Page: page, Limit: limit, Total: total},
	})
}

func (h *NotificationHandler) UnseenCount(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	subscriberID := c.Param("subscriberId")

	sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, subscriberID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "subscriber not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	count, err := h.notifRepo.UnseenCount(c.Request.Context(), envID, sub.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"count": count}})
}
