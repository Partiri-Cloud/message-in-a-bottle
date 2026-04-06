package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-box/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-box/internal/middleware"
	"github.com/partiri-cloud/message-in-a-box/internal/model"
	"github.com/partiri-cloud/message-in-a-box/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type WorkflowHandler struct {
	wfRepo *repository.WorkflowRepository
}

func NewWorkflowHandler(wfRepo *repository.WorkflowRepository) *WorkflowHandler {
	return &WorkflowHandler{wfRepo: wfRepo}
}

func (h *WorkflowHandler) Create(c *gin.Context) {
	var req dto.CreateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	wf := &model.Workflow{
		EnvironmentID: envID,
		Identifier:    req.Identifier,
		Name:          req.Name,
		Description:   req.Description,
		Tags:          req.Tags,
		Steps:         convertSteps(req.Steps),
		PreferenceDefaults: model.ChannelPrefs{
			Email:   req.PreferenceDefaults.Email,
			SMS:     req.PreferenceDefaults.SMS,
			Push:    req.PreferenceDefaults.Push,
			InApp:   req.PreferenceDefaults.InApp,
			Slack:   req.PreferenceDefaults.Slack,
			MSTeams: req.PreferenceDefaults.MSTeams,
		},
	}

	if err := h.wfRepo.Create(c.Request.Context(), wf); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "CONFLICT", "message": "workflow identifier already exists"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": wf})
}

func (h *WorkflowHandler) List(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	page, limit := dto.ParsePagination(c)

	workflows, total, err := h.wfRepo.FindMany(c.Request.Context(), envID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data: workflows,
		Meta: &dto.Meta{Page: page, Limit: limit, Total: total},
	})
}

func (h *WorkflowHandler) Get(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("workflowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid workflow ID"}})
		return
	}

	wf, err := h.wfRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "workflow not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": wf})
}

func (h *WorkflowHandler) Update(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("workflowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid workflow ID"}})
		return
	}

	var req dto.CreateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	wf := &model.Workflow{
		ID:            id,
		EnvironmentID: envID,
		Identifier:    req.Identifier,
		Name:          req.Name,
		Description:   req.Description,
		Tags:          req.Tags,
		Steps:         convertSteps(req.Steps),
		PreferenceDefaults: model.ChannelPrefs{
			Email:   req.PreferenceDefaults.Email,
			SMS:     req.PreferenceDefaults.SMS,
			Push:    req.PreferenceDefaults.Push,
			InApp:   req.PreferenceDefaults.InApp,
			Slack:   req.PreferenceDefaults.Slack,
			MSTeams: req.PreferenceDefaults.MSTeams,
		},
		IsActive: true,
	}

	if err := h.wfRepo.Update(c.Request.Context(), id, wf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": wf})
}

func (h *WorkflowHandler) SetStatus(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("workflowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid workflow ID"}})
		return
	}

	var req dto.WorkflowStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	if err := h.wfRepo.SetActive(c.Request.Context(), id, req.IsActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *WorkflowHandler) Delete(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("workflowId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid workflow ID"}})
		return
	}

	if err := h.wfRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func convertSteps(dtoSteps []dto.WorkflowStepDTO) []model.WorkflowStep {
	steps := make([]model.WorkflowStep, len(dtoSteps))
	for i, s := range dtoSteps {
		step := model.WorkflowStep{
			ID:             bson.NewObjectID(),
			Type:           s.Type,
			Order:          s.Order,
			DefaultEnabled: s.DefaultEnabled,
		}
		if s.Template != nil {
			step.Template = &model.StepTemplate{
				Subject: s.Template.Subject,
				Body:    s.Template.Body,
				Content: s.Template.Content,
			}
		}
		if s.DigestConfig != nil {
			step.DigestConfig = &model.DigestConfig{
				Amount:    s.DigestConfig.Amount,
				Unit:      s.DigestConfig.Unit,
				DigestKey: s.DigestConfig.DigestKey,
			}
		}
		if s.DelayConfig != nil {
			step.DelayConfig = &model.DelayConfig{
				Amount: s.DelayConfig.Amount,
				Unit:   s.DelayConfig.Unit,
			}
		}
		for _, cond := range s.Conditions {
			step.Conditions = append(step.Conditions, model.StepCondition{
				Field:    cond.Field,
				Operator: cond.Operator,
				Value:    cond.Value,
			})
		}
		steps[i] = step
	}
	return steps
}
