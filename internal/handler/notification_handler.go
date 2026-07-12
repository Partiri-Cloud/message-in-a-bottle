package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-bottle/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-bottle/internal/middleware"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// notificationRepository narrows *repository.NotificationRepository to the
// methods NotificationHandler needs, so tests can inject stubs into the real
// handler.
type notificationRepository interface {
	FindMany(ctx context.Context, envID bson.ObjectID, page, limit int) ([]model.Notification, int64, error)
	FindByID(ctx context.Context, envID, id bson.ObjectID) (*model.Notification, error)
	FindFeed(ctx context.Context, envID, subscriberID bson.ObjectID, filter repository.FeedFilter, page, limit int) ([]model.Notification, int64, error)
	MarkSeen(ctx context.Context, envID, subID, id bson.ObjectID) error
	MarkRead(ctx context.Context, envID, subID, id bson.ObjectID) error
	MarkArchived(ctx context.Context, envID, subID, id bson.ObjectID) error
	BulkMarkRead(ctx context.Context, envID, subID bson.ObjectID, ids []bson.ObjectID) error
	BulkMarkSeen(ctx context.Context, envID, subID bson.ObjectID, ids []bson.ObjectID) error
	UnseenCount(ctx context.Context, envID, subscriberID bson.ObjectID) (int64, error)
}

// activityRepository narrows *repository.ActivityRepository to the method
// NotificationHandler needs.
type activityRepository interface {
	FindMany(ctx context.Context, envID bson.ObjectID, page, limit int) ([]model.ActivityLog, int64, error)
}

// subscriberLookupRepository narrows *repository.SubscriberRepository to the
// method NotificationHandler needs.
type subscriberLookupRepository interface {
	FindBySubscriberID(ctx context.Context, envID bson.ObjectID, subscriberID string) (*model.Subscriber, error)
}

type NotificationHandler struct {
	notifRepo    notificationRepository
	activityRepo activityRepository
	subRepo      subscriberLookupRepository
}

func NewNotificationHandler(notifRepo *repository.NotificationRepository, activityRepo *repository.ActivityRepository, subRepo *repository.SubscriberRepository) *NotificationHandler {
	return &NotificationHandler{notifRepo: notifRepo, activityRepo: activityRepo, subRepo: subRepo}
}

func (h *NotificationHandler) List(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	page, limit := dto.ParsePagination(c)

	notifs, total, err := h.notifRepo.FindMany(c.Request.Context(), envID, page, limit)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data: notifs,
		Meta: &dto.Meta{Page: page, Limit: limit, Total: total},
	})
}

func (h *NotificationHandler) Get(c *gin.Context) {
	id, ok := parseObjectIDParam(c, "id", "notification")
	if !ok {
		return
	}

	envID := middleware.GetEnvironmentID(c)
	notif, err := h.notifRepo.FindByID(c.Request.Context(), envID, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "notification not found"}})
			return
		}
		internalError(c, err)
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
		respondSubscriberErr(c, err)
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
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data: notifs,
		Meta: &dto.Meta{Page: page, Limit: limit, Total: total},
	})
}

func (h *NotificationHandler) MarkSeen(c *gin.Context) {
	notifID, ok := parseObjectIDParam(c, "notifId", "notification")
	if !ok {
		return
	}

	envID := middleware.GetEnvironmentID(c)
	sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, c.Param("subscriberId"))
	if err != nil {
		respondSubscriberErr(c, err)
		return
	}

	if err := h.notifRepo.MarkSeen(c.Request.Context(), envID, sub.ID, notifID); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "notification not found"}})
			return
		}
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	notifID, ok := parseObjectIDParam(c, "notifId", "notification")
	if !ok {
		return
	}

	envID := middleware.GetEnvironmentID(c)
	sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, c.Param("subscriberId"))
	if err != nil {
		respondSubscriberErr(c, err)
		return
	}

	if err := h.notifRepo.MarkRead(c.Request.Context(), envID, sub.ID, notifID); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "notification not found"}})
			return
		}
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *NotificationHandler) Archive(c *gin.Context) {
	notifID, ok := parseObjectIDParam(c, "notifId", "notification")
	if !ok {
		return
	}

	envID := middleware.GetEnvironmentID(c)
	sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, c.Param("subscriberId"))
	if err != nil {
		respondSubscriberErr(c, err)
		return
	}

	if err := h.notifRepo.MarkArchived(c.Request.Context(), envID, sub.ID, notifID); err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "notification not found"}})
			return
		}
		internalError(c, err)
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

	envID := middleware.GetEnvironmentID(c)
	sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, c.Param("subscriberId"))
	if err != nil {
		respondSubscriberErr(c, err)
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
		bulkErr = h.notifRepo.BulkMarkRead(c.Request.Context(), envID, sub.ID, ids)
	case "seen":
		bulkErr = h.notifRepo.BulkMarkSeen(c.Request.Context(), envID, sub.ID, ids)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "action must be 'read' or 'seen'"}})
		return
	}

	if bulkErr != nil {
		internalError(c, bulkErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *NotificationHandler) Activity(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	page, limit := dto.ParsePagination(c)

	logs, total, err := h.activityRepo.FindMany(c.Request.Context(), envID, page, limit)
	if err != nil {
		internalError(c, err)
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
		respondSubscriberErr(c, err)
		return
	}

	count, err := h.notifRepo.UnseenCount(c.Request.Context(), envID, sub.ID)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"count": count}})
}
