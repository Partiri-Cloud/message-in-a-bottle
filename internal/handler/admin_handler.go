package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type AdminHandler struct {
	envRepo *repository.EnvironmentRepository
}

func NewAdminHandler(envRepo *repository.EnvironmentRepository) *AdminHandler {
	return &AdminHandler{envRepo: envRepo}
}

func (h *AdminHandler) CreateEnvironment(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		Identifier string `json:"identifier" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	rawKey, keyHash, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to generate API key"}})
		return
	}

	env := &model.Environment{
		Name:       req.Name,
		Identifier: req.Identifier,
		APIKeys: []model.APIKey{
			{
				KeyHash:     keyHash,
				Name:        "default",
				Permissions: allPermissions(),
				CreatedAt:   time.Now(),
				IsActive:    true,
			},
		},
	}

	if err := h.envRepo.Create(c.Request.Context(), env); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "CONFLICT", "message": fmt.Sprintf("environment with identifier %q already exists", req.Identifier)}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to create environment"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"id":         env.ID.Hex(),
		"name":       env.Name,
		"identifier": env.Identifier,
		"apiKey":     rawKey,
	}})
}

func (h *AdminHandler) ListEnvironments(c *gin.Context) {
	envs, err := h.envRepo.FindAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to list environments"}})
		return
	}

	result := make([]gin.H, 0, len(envs))
	for _, env := range envs {
		keys := make([]gin.H, 0, len(env.APIKeys))
		for _, k := range env.APIKeys {
			keys = append(keys, gin.H{
				"name":       k.Name,
				"isActive":   k.IsActive,
				"createdAt":  k.CreatedAt,
				"expiresAt":  k.ExpiresAt,
				"lastUsedAt": k.LastUsedAt,
			})
		}
		result = append(result, gin.H{
			"id":         env.ID.Hex(),
			"name":       env.Name,
			"identifier": env.Identifier,
			"apiKeys":    keys,
			"createdAt":  env.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *AdminHandler) AddAPIKey(c *gin.Context) {
	identifier := c.Param("identifier")

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	env, err := h.envRepo.FindByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": fmt.Sprintf("environment %q not found", identifier)}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to find environment"}})
		return
	}

	rawKey, keyHash, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to generate API key"}})
		return
	}

	apiKey := model.APIKey{
		KeyHash:     keyHash,
		Name:        req.Name,
		Permissions: allPermissions(),
		CreatedAt:   time.Now(),
		IsActive:    true,
	}

	if err := h.envRepo.AddAPIKey(c.Request.Context(), env.ID, apiKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to add API key"}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"name":   req.Name,
		"apiKey": rawKey,
	}})
}

func generateAPIKey() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("failed to generate random key: %w", err)
	}
	rawKey := "mib_" + hex.EncodeToString(buf)
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])
	return rawKey, keyHash, nil
}

func allPermissions() []string {
	return []string{
		"subscribers:read", "subscribers:write",
		"topics:read", "topics:write",
		"workflows:read", "workflows:write",
		"integrations:read", "integrations:write",
		"templates:read", "templates:write",
		"notifications:read", "notifications:trigger",
		"preferences:read", "preferences:write",
		"activity:read",
	}
}
