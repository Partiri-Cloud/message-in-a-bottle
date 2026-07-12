package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
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

// respondSubscriberErr maps a subscriber lookup failure to the standard
// 404/500 contract shared by every subscriber-scoped endpoint.
func respondSubscriberErr(c *gin.Context, err error) {
	if errors.Is(err, mongo.ErrNoDocuments) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "subscriber not found"}})
		return
	}
	internalError(c, err)
}

// parseObjectIDParam parses the named URL parameter as an ObjectID, responding
// with the standard validation error ("invalid <noun> ID") when it is malformed.
func parseObjectIDParam(c *gin.Context, param, noun string) (bson.ObjectID, bool) {
	id, err := bson.ObjectIDFromHex(c.Param(param))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": "invalid " + noun + " ID"}})
		return bson.ObjectID{}, false
	}
	return id, true
}
