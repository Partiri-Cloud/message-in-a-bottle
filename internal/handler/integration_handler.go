package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-box/internal/crypto"
	"github.com/partiri-cloud/message-in-a-box/internal/handler/dto"
	"github.com/partiri-cloud/message-in-a-box/internal/middleware"
	"github.com/partiri-cloud/message-in-a-box/internal/model"
	"github.com/partiri-cloud/message-in-a-box/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type IntegrationHandler struct {
	intgRepo      *repository.IntegrationRepository
	encryptionKey []byte
}

func NewIntegrationHandler(intgRepo *repository.IntegrationRepository, encryptionKey []byte) *IntegrationHandler {
	return &IntegrationHandler{intgRepo: intgRepo, encryptionKey: encryptionKey}
}

func (h *IntegrationHandler) Create(c *gin.Context) {
	var req dto.CreateIntegrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)

	credJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid credentials"}})
		return
	}

	encrypted, err := crypto.Encrypt(credJSON, h.encryptionKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to encrypt credentials"}})
		return
	}

	intg := &model.Integration{
		EnvironmentID: envID,
		Channel:       req.Channel,
		ProviderID:    req.ProviderID,
		Name:          req.Name,
		Credentials:   encrypted,
		IsPrimary:     req.IsPrimary,
		IsActive:      true,
		Metadata: model.IntegrationMeta{
			SenderName:  req.Metadata.SenderName,
			SenderEmail: req.Metadata.SenderEmail,
		},
	}

	if err := h.intgRepo.Create(c.Request.Context(), intg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	// Don't return credentials in response
	intg.Credentials = nil
	c.JSON(http.StatusCreated, gin.H{"data": intg})
}

func (h *IntegrationHandler) List(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	page, limit := dto.ParsePagination(c)

	integrations, total, err := h.intgRepo.FindMany(c.Request.Context(), envID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	// Strip credentials from response
	for i := range integrations {
		integrations[i].Credentials = nil
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data: integrations,
		Meta: &dto.Meta{Page: page, Limit: limit, Total: total},
	})
}

func (h *IntegrationHandler) Update(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid integration ID"}})
		return
	}

	var req dto.UpdateIntegrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)

	intg, err := h.intgRepo.FindByID(c.Request.Context(), envID, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "integration not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to retrieve integration"}})
		return
	}

	if req.Name != "" {
		intg.Name = req.Name
	}
	if req.IsActive != nil {
		intg.IsActive = *req.IsActive
	}
	if req.Metadata.SenderName != "" {
		intg.Metadata.SenderName = req.Metadata.SenderName
	}
	if req.Metadata.SenderEmail != "" {
		intg.Metadata.SenderEmail = req.Metadata.SenderEmail
	}
	if req.Credentials != nil {
		credJSON, _ := json.Marshal(req.Credentials)
		encrypted, err := crypto.Encrypt(credJSON, h.encryptionKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to encrypt credentials"}})
			return
		}
		intg.Credentials = encrypted
	}

	if err := h.intgRepo.Update(c.Request.Context(), envID, id, intg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to update integration"}})
		return
	}

	intg.Credentials = nil
	c.JSON(http.StatusOK, gin.H{"data": intg})
}

func (h *IntegrationHandler) Delete(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid integration ID"}})
		return
	}

	envID := middleware.GetEnvironmentID(c)

	if err := h.intgRepo.Delete(c.Request.Context(), envID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to delete integration"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *IntegrationHandler) SetPrimary(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid integration ID"}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	if err := h.intgRepo.SetPrimary(c.Request.Context(), envID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "an internal error occurred"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}
