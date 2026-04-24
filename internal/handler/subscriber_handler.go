package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-bottle/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-bottle/internal/middleware"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type SubscriberHandler struct {
	subRepo *repository.SubscriberRepository
	tsRepo  *repository.TopicSubscriberRepository
}

func NewSubscriberHandler(subRepo *repository.SubscriberRepository, tsRepo *repository.TopicSubscriberRepository) *SubscriberHandler {
	return &SubscriberHandler{subRepo: subRepo, tsRepo: tsRepo}
}

func (h *SubscriberHandler) Create(c *gin.Context) {
	var req dto.CreateSubscriberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	sub := &model.Subscriber{
		SubscriberID: req.SubscriberID,
		Email:        req.Email,
		Phone:        req.Phone,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Avatar:       req.Avatar,
		Locale:       req.Locale,
		Timezone:     req.Timezone,
		Data:         req.Data,
	}
	if req.Channels != nil {
		if req.Channels.Push != nil {
			sub.Channels.Push = model.PushChannelConfig{
				FCMTokens:  req.Channels.Push.FCMTokens,
				APNSTokens: req.Channels.Push.APNSTokens,
			}
		}
		if req.Channels.Slack != nil {
			sub.Channels.Slack = model.SlackConfig{WebhookURL: req.Channels.Slack.WebhookURL}
		}
		if req.Channels.MSTeams != nil {
			sub.Channels.MSTeams = model.MSTeamsConfig{WebhookURL: req.Channels.MSTeams.WebhookURL}
		}
	}
	if sub.Locale == "" {
		sub.Locale = "en"
	}

	if err := h.subRepo.Upsert(c.Request.Context(), envID, sub); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": sub})
}

func (h *SubscriberHandler) BulkCreate(c *gin.Context) {
	var req dto.BulkSubscribersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	subs := make([]model.Subscriber, len(req.Subscribers))
	for i, s := range req.Subscribers {
		subs[i] = model.Subscriber{
			SubscriberID: s.SubscriberID,
			Email:        s.Email,
			Phone:        s.Phone,
			FirstName:    s.FirstName,
			LastName:     s.LastName,
			Avatar:       s.Avatar,
			Locale:       s.Locale,
			Timezone:     s.Timezone,
			Data:         s.Data,
		}
		if subs[i].Locale == "" {
			subs[i].Locale = "en"
		}
	}

	if err := h.subRepo.BulkUpsert(c.Request.Context(), envID, subs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"count": len(subs)}})
}

func (h *SubscriberHandler) Get(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{"data": sub})
}

func (h *SubscriberHandler) Update(c *gin.Context) {
	var req dto.UpdateSubscriberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	subscriberID := c.Param("subscriberId")

	update := make(map[string]any)
	if req.Email != nil {
		update["email"] = *req.Email
	}
	if req.Phone != nil {
		update["phone"] = *req.Phone
	}
	if req.FirstName != nil {
		update["firstName"] = *req.FirstName
	}
	if req.LastName != nil {
		update["lastName"] = *req.LastName
	}
	if req.Avatar != nil {
		update["avatar"] = *req.Avatar
	}
	if req.Locale != nil {
		update["locale"] = *req.Locale
	}
	if req.Timezone != nil {
		update["timezone"] = *req.Timezone
	}
	if req.Data != nil {
		update["data"] = req.Data
	}

	if err := h.subRepo.Update(c.Request.Context(), envID, subscriberID, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *SubscriberHandler) Delete(c *gin.Context) {
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

	// Remove from all topics
	if err := h.tsRepo.DeleteBySubscriber(c.Request.Context(), sub.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	if err := h.subRepo.Delete(c.Request.Context(), envID, subscriberID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}
