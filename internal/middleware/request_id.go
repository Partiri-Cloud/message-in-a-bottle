package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/partiri-cloud/message-in-a-bottle/internal/logging"
)

const (
	// ContextKeyRequestID is the gin context key under which the per-request
	// correlation ID is stored.
	ContextKeyRequestID = "requestId"
	// HeaderRequestID is the HTTP header used to read an inbound request ID and
	// to echo the resolved one back to the client.
	HeaderRequestID = "X-Request-Id"
)

// RequestID resolves a correlation ID for every request: it reuses an inbound
// X-Request-Id header when present, otherwise generates one. The value is stored
// in the gin context and echoed on the response so callers (and downstream
// workers) can correlate logs across the request lifecycle.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(ContextKeyRequestID, id)
		c.Writer.Header().Set(HeaderRequestID, id)
		c.Request = c.Request.WithContext(logging.WithRequestID(c.Request.Context(), id))
		c.Next()
	}
}

// GetRequestID returns the correlation ID for the current request, or "" if unset.
func GetRequestID(c *gin.Context) string {
	v, ok := c.Get(ContextKeyRequestID)
	if !ok {
		return ""
	}
	id, _ := v.(string)
	return id
}
