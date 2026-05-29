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

type TopicHandler struct {
	topicRepo *repository.TopicRepository
	tsRepo    *repository.TopicSubscriberRepository
	subRepo   *repository.SubscriberRepository
}

func NewTopicHandler(topicRepo *repository.TopicRepository, tsRepo *repository.TopicSubscriberRepository, subRepo *repository.SubscriberRepository) *TopicHandler {
	return &TopicHandler{topicRepo: topicRepo, tsRepo: tsRepo, subRepo: subRepo}
}

func (h *TopicHandler) Create(c *gin.Context) {
	var req dto.CreateTopicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	topic := &model.Topic{
		EnvironmentID: envID,
		Key:           req.Key,
		Name:          req.Name,
	}

	if err := h.topicRepo.Create(c.Request.Context(), topic); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "CONFLICT", "message": "topic key already exists"}})
			return
		}
		internalError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": topic})
}

func (h *TopicHandler) List(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	page, limit := dto.ParsePagination(c)
	keyPrefix := c.Query("keyPrefix")

	topics, total, err := h.topicRepo.FindMany(c.Request.Context(), envID, keyPrefix, page, limit)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data: topics,
		Meta: &dto.Meta{Page: page, Limit: limit, Total: total},
	})
}

func (h *TopicHandler) Get(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	topicKey := c.Param("topicKey")

	topic, err := h.topicRepo.FindByKey(c.Request.Context(), envID, topicKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "topic not found"}})
			return
		}
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": topic})
}

func (h *TopicHandler) Update(c *gin.Context) {
	var req dto.UpdateTopicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	topicKey := c.Param("topicKey")

	if err := h.topicRepo.UpdateName(c.Request.Context(), envID, topicKey, req.Name); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *TopicHandler) Delete(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	topicKey := c.Param("topicKey")

	topic, err := h.topicRepo.FindByKey(c.Request.Context(), envID, topicKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "topic not found"}})
			return
		}
		internalError(c, err)
		return
	}

	if err := h.tsRepo.DeleteByTopic(c.Request.Context(), topic.ID); err != nil {
		internalError(c, err)
		return
	}

	if err := h.topicRepo.Delete(c.Request.Context(), envID, topicKey); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *TopicHandler) AddSubscribers(c *gin.Context) {
	var req dto.TopicSubscribersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	topicKey := c.Param("topicKey")

	topic, err := h.topicRepo.FindOrCreate(c.Request.Context(), envID, topicKey)
	if err != nil {
		internalError(c, err)
		return
	}

	var topicSubs []model.TopicSubscriber
	for _, subID := range req.SubscriberIDs {
		sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, subID)
		if err != nil {
			continue // skip non-existent subscribers
		}
		topicSubs = append(topicSubs, model.TopicSubscriber{
			SubscriberID:         sub.ID,
			ExternalSubscriberID: subID,
		})
	}

	if err := h.tsRepo.BulkAdd(c.Request.Context(), envID, topic.ID, topicSubs); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *TopicHandler) RemoveSubscribers(c *gin.Context) {
	var req dto.TopicSubscribersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	envID := middleware.GetEnvironmentID(c)
	topicKey := c.Param("topicKey")

	topic, err := h.topicRepo.FindByKey(c.Request.Context(), envID, topicKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "topic not found"}})
			return
		}
		internalError(c, err)
		return
	}

	var subOIDs []bson.ObjectID
	for _, subID := range req.SubscriberIDs {
		sub, err := h.subRepo.FindBySubscriberID(c.Request.Context(), envID, subID)
		if err != nil {
			continue
		}
		subOIDs = append(subOIDs, sub.ID)
	}

	if err := h.tsRepo.BulkRemove(c.Request.Context(), topic.ID, subOIDs); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"acknowledged": true}})
}

func (h *TopicHandler) ListSubscribers(c *gin.Context) {
	envID := middleware.GetEnvironmentID(c)
	topicKey := c.Param("topicKey")
	page, limit := dto.ParsePagination(c)

	topic, err := h.topicRepo.FindByKey(c.Request.Context(), envID, topicKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "topic not found"}})
			return
		}
		internalError(c, err)
		return
	}

	subs, total, err := h.tsRepo.FindByTopic(c.Request.Context(), topic.ID, page, limit)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.PaginatedResponse{
		Data: subs,
		Meta: &dto.Meta{Page: page, Limit: limit, Total: total},
	})
}
