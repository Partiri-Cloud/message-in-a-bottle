package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-bottle/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-bottle/internal/middleware"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"github.com/partiri-cloud/message-in-a-bottle/internal/service"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type TemplateHandler struct {
	tmplRepo *repository.TemplateRepository
	tmplSvc  *service.TemplateService
}

func NewTemplateHandler(tmplRepo *repository.TemplateRepository, tmplSvc *service.TemplateService) *TemplateHandler {
	return &TemplateHandler{tmplRepo: tmplRepo, tmplSvc: tmplSvc}
}

func (h *TemplateHandler) Create(c *gin.Context) {
	var req dto.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	tmpl := &model.TransactionalTemplate{
		EnvironmentID: envID,
		Identifier:    req.Identifier,
		Name:          req.Name,
		Channel:       req.Channel,
		Subject:       req.Subject,
		Body:          req.Body,
		DefaultLocale: req.DefaultLocale,
		Variables:     req.Variables,
	}
	if tmpl.DefaultLocale == "" {
		tmpl.DefaultLocale = "en"
	}

	if err := h.tmplRepo.Create(c.Request.Context(), tmpl); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "CONFLICT", "message": "template identifier already exists"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": tmpl})
}

func (h *TemplateHandler) List(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	page, limit := dto.ParsePagination(c)

	tmpls, total, err := h.tmplRepo.FindMany(c.Request.Context(), envID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data: tmpls,
		Meta: &dto.Meta{Page: page, Limit: limit, Total: total},
	})
}

func (h *TemplateHandler) Get(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	identifier := c.Param("identifier")

	tmpl, err := h.tmplRepo.FindByIdentifier(c.Request.Context(), envID, identifier)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "template not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tmpl})
}

func (h *TemplateHandler) Update(c *gin.Context) {
	var req dto.UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	identifier := c.Param("identifier")

	tmpl, err := h.tmplRepo.FindByIdentifier(c.Request.Context(), envID, identifier)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "template not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	if req.Name != "" {
		tmpl.Name = req.Name
	}
	if req.Subject != nil {
		tmpl.Subject = req.Subject
	}
	if req.Body != nil {
		tmpl.Body = req.Body
	}
	if req.DefaultLocale != "" {
		tmpl.DefaultLocale = req.DefaultLocale
	}
	if req.Variables != nil {
		tmpl.Variables = req.Variables
	}
	if req.IsActive != nil {
		tmpl.IsActive = *req.IsActive
	}

	if err := h.tmplRepo.Update(c.Request.Context(), envID, identifier, tmpl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tmpl})
}

func (h *TemplateHandler) Delete(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	identifier := c.Param("identifier")

	if err := h.tmplRepo.Delete(c.Request.Context(), envID, identifier); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *TemplateHandler) Send(c *gin.Context) {
	var req dto.SendTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	identifier := c.Param("identifier")

	if err := h.tmplSvc.Send(c.Request.Context(), envID, identifier, req.To.SubscriberID, req.Payload, req.Locale); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"acknowledged": true}})
}
