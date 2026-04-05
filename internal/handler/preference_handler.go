package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/partiri/message-in-a-bottle/internal/handler/dto"
	"github.com/partiri/message-in-a-bottle/internal/middleware"
	"github.com/partiri/message-in-a-bottle/internal/model"
	"github.com/partiri/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type PreferenceHandler struct {
	prefRepo *repository.PreferenceRepository
	subRepo  *repository.SubscriberRepository
}

func NewPreferenceHandler(prefRepo *repository.PreferenceRepository, subRepo *repository.SubscriberRepository) *PreferenceHandler {
	return &PreferenceHandler{prefRepo: prefRepo, subRepo: subRepo}
}

func (h *PreferenceHandler) GetAll(c *gin.Context) {
	if !middleware.EnforceSubscriberScope(c) {
		return
	}
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

	prefs, err := h.prefRepo.FindBySubscriber(c.Request.Context(), envID, sub.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": prefs})
}

func (h *PreferenceHandler) UpdateGlobal(c *gin.Context) {
	if !middleware.EnforceSubscriberScope(c) {
		return
	}
	var req dto.UpdatePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

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

	pref := &model.SubscriberPreference{
		EnvironmentID: envID,
		SubscriberID:  sub.ID,
		WorkflowID:    nil, // global
		Channels: model.ChannelPrefs{
			Email:   req.Channels.Email,
			SMS:     req.Channels.SMS,
			Push:    req.Channels.Push,
			InApp:   req.Channels.InApp,
			Slack:   req.Channels.Slack,
			MSTeams: req.Channels.MSTeams,
		},
	}

	if err := h.prefRepo.Upsert(c.Request.Context(), pref); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": pref})
}

func (h *PreferenceHandler) UpdateWorkflow(c *gin.Context) {
	if !middleware.EnforceSubscriberScope(c) {
		return
	}
	var req dto.UpdatePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	subscriberID := c.Param("subscriberId")
	workflowID, err := bson.ObjectIDFromHex(c.Param("workflowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid workflow ID"}})
		return
	}

	sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, subscriberID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "subscriber not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	pref := &model.SubscriberPreference{
		EnvironmentID: envID,
		SubscriberID:  sub.ID,
		WorkflowID:    &workflowID,
		Channels: model.ChannelPrefs{
			Email:   req.Channels.Email,
			SMS:     req.Channels.SMS,
			Push:    req.Channels.Push,
			InApp:   req.Channels.InApp,
			Slack:   req.Channels.Slack,
			MSTeams: req.Channels.MSTeams,
		},
	}

	if err := h.prefRepo.Upsert(c.Request.Context(), pref); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": pref})
}
