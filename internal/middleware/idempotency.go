package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/partiri/message-in-a-bottle/internal/repository"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func Idempotency(notifRepo *repository.NotificationRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Idempotency is checked in the trigger service via transactionId.
		// This middleware is a placeholder for future request-level idempotency.
		c.Next()
	}
}

func CheckTransactionID(notifRepo *repository.NotificationRepository, c *gin.Context, transactionID string) (bool, error) {
	if transactionID == "" {
		return false, nil
	}
	envID := GetEnvironmentID(c)
	existing, err := notifRepo.FindByTransactionID(c.Request.Context(), envID, transactionID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, err
	}
	c.JSON(http.StatusConflict, gin.H{
		"error": gin.H{
			"code":    "CONFLICT",
			"message": "duplicate transactionId",
		},
		"data": gin.H{
			"notificationId": existing.ID.Hex(),
		},
	})
	return true, nil
}
