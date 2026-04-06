package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/partiri-cloud/message-in-a-box/internal/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	ContextKeyEnvironmentID = "environmentId"
	ContextKeyPermissions   = "permissions"
	ContextKeyAPIKeyHash    = "apiKeyHash"
	maxDebounceEntries      = 10000
)

func AuthMiddleware(envRepo *repository.EnvironmentRepository) gin.HandlerFunc {
	var mu sync.Mutex
	lastUpdated := make(map[string]time.Time)

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "missing Authorization header",
			}})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "ApiKey" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "invalid Authorization format, expected: ApiKey <key>",
			}})
			return
		}

		rawKey := parts[1]
		hash := sha256.Sum256([]byte(rawKey))
		keyHash := hex.EncodeToString(hash[:])

		env, apiKey, err := envRepo.FindByAPIKeyHash(c.Request.Context(), keyHash)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "invalid API key",
				}})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "authentication error",
			}})
			return
		}

		if !apiKey.IsActive {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "API key is inactive",
			}})
			return
		}

		if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "API key has expired",
			}})
			return
		}

		c.Set(ContextKeyEnvironmentID, env.ID)
		c.Set(ContextKeyPermissions, apiKey.Permissions)
		c.Set(ContextKeyAPIKeyHash, keyHash)

		// Debounce lastUsedAt update (only if >5 min since last)
		mu.Lock()
		last, ok := lastUpdated[keyHash]
		shouldUpdate := !ok || time.Since(last) > 5*time.Minute
		if shouldUpdate {
			lastUpdated[keyHash] = time.Now()
			// Evict oldest entries if map grows too large
			if len(lastUpdated) > maxDebounceEntries {
				for k := range lastUpdated {
					delete(lastUpdated, k)
					break
				}
			}
		}
		mu.Unlock()

		if shouldUpdate {
			envIDCopy := env.ID
			go envRepo.UpdateLastUsedAt(context.Background(), envIDCopy, keyHash, time.Now())
		}

		c.Next()
	}
}

func GetEnvironmentID(c *gin.Context) bson.ObjectID {
	v, _ := c.Get(ContextKeyEnvironmentID)
	return v.(bson.ObjectID)
}

func GetPermissions(c *gin.Context) []string {
	v, _ := c.Get(ContextKeyPermissions)
	return v.([]string)
}
