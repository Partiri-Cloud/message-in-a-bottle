package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const ContextKeySubscriberID = "scopedSubscriberId"

type SubscriberTokenPayload struct {
	SubscriberID string `json:"subscriberId"`
	Timestamp    int64  `json:"timestamp"`
}

var (
	ErrInvalidTokenFormat    = errors.New("invalid subscriber token format")
	ErrInvalidTokenEncoding  = errors.New("invalid subscriber token encoding")
	ErrInvalidTokenSignature = errors.New("invalid subscriber token signature")
	ErrInvalidTokenPayload   = errors.New("invalid subscriber token payload")
	ErrTokenExpired          = errors.New("subscriber token expired")
)

// ValidateSubscriberToken validates an HMAC-signed subscriber token and returns the parsed payload.
func ValidateSubscriberToken(token, hmacSecret string) (*SubscriberTokenPayload, error) {
	if hmacSecret == "" {
		return nil, errors.New("subscriber HMAC secret not configured")
	}

	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidTokenFormat
	}

	payloadBytes, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidTokenEncoding
	}

	// Verify HMAC (constant-time comparison)
	mac := hmac.New(sha256.New, []byte(hmacSecret))
	mac.Write(payloadBytes)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedSig), []byte(parts[1])) {
		return nil, ErrInvalidTokenSignature
	}

	var payload SubscriberTokenPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, ErrInvalidTokenPayload
	}

	// Token expires after 24 hours
	tokenTime := time.UnixMilli(payload.Timestamp)
	if time.Since(tokenTime) > 24*time.Hour {
		return nil, ErrTokenExpired
	}

	return &payload, nil
}

func SubscriberScope(hmacSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Subscriber-Token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "missing X-Subscriber-Token header",
			}})
			return
		}

		payload, err := ValidateSubscriberToken(token, hmacSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": err.Error(),
			}})
			return
		}

		if urlID := c.Param("subscriberId"); urlID != "" && urlID != payload.SubscriberID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "subscriber token does not match requested subscriber",
			}})
			return
		}

		c.Set(ContextKeySubscriberID, payload.SubscriberID)
		c.Next()
	}
}

func GetScopedSubscriberID(c *gin.Context) string {
	v, _ := c.Get(ContextKeySubscriberID)
	return v.(string)
}
