package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Logging emits one structured log line per HTTP request after it completes,
// including correlation/tenant context and any errors handlers attached via
// c.Error(). It is the production replacement for gin's default text logger and
// is what surfaces otherwise-generic 5xx responses with their underlying cause.
func Logging(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int64("latencyMs", time.Since(start).Milliseconds()),
			slog.String("clientIp", c.ClientIP()),
		}

		if rid := GetRequestID(c); rid != "" {
			attrs = append(attrs, slog.String("requestId", rid))
		}
		if envID, ok := c.Get(ContextKeyEnvironmentID); ok {
			if id, ok := envID.(bson.ObjectID); ok {
				attrs = append(attrs, slog.String("environmentId", id.Hex()))
			}
		}
		// Surface the real cause(s) of failures that handlers attached via c.Error.
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
		}

		switch {
		case status >= 500:
			logger.Error("request failed", attrs...)
		case status >= 400:
			logger.Warn("request rejected", attrs...)
		default:
			logger.Info("request", attrs...)
		}
	}
}
