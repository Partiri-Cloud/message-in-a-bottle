package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		perms := GetPermissions(c)
		if !slices.Contains(perms, permission) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "missing permission: " + permission,
			}})
			return
		}
		c.Next()
	}
}
