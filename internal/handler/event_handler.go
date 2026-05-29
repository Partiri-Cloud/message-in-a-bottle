package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-bottle/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-bottle/internal/middleware"
	"github.com/partiri-cloud/message-in-a-bottle/internal/service"
)

type EventHandler struct {
	triggerSvc *service.TriggerService
}

func NewEventHandler(triggerSvc *service.TriggerService) *EventHandler {
	return &EventHandler{triggerSvc: triggerSvc}
}

func (h *EventHandler) Trigger(c *gin.Context) {
	var req dto.TriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)

	result, err := h.triggerSvc.Trigger(c.Request.Context(), envID, &req)
	if err != nil {
		if err == service.ErrDuplicateTransaction {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "CONFLICT", "message": "duplicate transactionId"}})
			return
		}
		if err == service.ErrWorkflowNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "workflow not found"}})
			return
		}
		internalError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": result})
}

func (h *EventHandler) BulkTrigger(c *gin.Context) {
	var req dto.BulkTriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)

	var results []any
	for _, event := range req.Events {
		result, err := h.triggerSvc.Trigger(c.Request.Context(), envID, &event)
		if err != nil {
			results = append(results, gin.H{"error": gin.H{"code": "TRIGGER_FAILED", "message": err.Error()}})
			continue
		}
		results = append(results, result)
	}

	c.JSON(http.StatusCreated, gin.H{"data": results})
}

func (h *EventHandler) Broadcast(c *gin.Context) {
	var req dto.BroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)

	result, err := h.triggerSvc.Broadcast(c.Request.Context(), envID, &req)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": result})
}
