package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// internalError records the underlying cause on the gin context (so the logging
// middleware emits it with the request's correlation/tenant context) and returns
// the generic 500 body to the client. The client-facing contract is unchanged —
// callers never see internal details — but operators get a traceable log line.
func internalError(c *gin.Context, err error) {
	internalErrorMsg(c, err, "an internal error occurred")
}

// internalErrorMsg behaves like internalError but preserves a specific
// client-facing message for endpoints that return one.
func internalErrorMsg(c *gin.Context, err error, message string) {
	if err != nil {
		_ = c.Error(err)
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{
		"code":    "INTERNAL_ERROR",
		"message": message,
	}})
}
