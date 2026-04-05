package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupPermissionTest(permissions []string) (*gin.Engine, *httptest.ResponseRecorder) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ContextKeyPermissions, permissions)
		c.Next()
	})
	return r, httptest.NewRecorder()
}

func TestRequirePermission_HasPermission(t *testing.T) {
	r, w := setupPermissionTest([]string{"subscribers:read", "subscribers:write"})
	r.GET("/test", RequirePermission("subscribers:read"), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePermission_MissingPermission(t *testing.T) {
	r, w := setupPermissionTest([]string{"subscribers:read"})
	r.GET("/test", RequirePermission("workflows:write"), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "missing permission")
}

func TestRequirePermission_EmptyPermissions(t *testing.T) {
	r, w := setupPermissionTest([]string{})
	r.GET("/test", RequirePermission("subscribers:read"), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
