package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

const testHMACSecret = "test-secret-key-for-hmac"

func generateToken(subscriberID string, timestamp int64) string {
	payload := fmt.Sprintf(`{"subscriberId":"%s","timestamp":%d}`, subscriberID, timestamp)
	mac := hmac.New(sha256.New, []byte(testHMACSecret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return base64.StdEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func setupScopeTest() (*gin.Engine, *httptest.ResponseRecorder) {
	r := gin.New()
	return r, httptest.NewRecorder()
}

func TestSubscriberScope_ValidToken(t *testing.T) {
	r, w := setupScopeTest()
	token := generateToken("usr_123", time.Now().UnixMilli())

	var gotSubID string
	r.GET("/test", SubscriberScope(testHMACSecret), func(c *gin.Context) {
		gotSubID = GetScopedSubscriberID(c)
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Subscriber-Token", token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "usr_123", gotSubID)
}

func TestSubscriberScope_MissingHeader(t *testing.T) {
	r, w := setupScopeTest()
	r.GET("/test", SubscriberScope(testHMACSecret), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSubscriberScope_InvalidFormat(t *testing.T) {
	r, w := setupScopeTest()
	r.GET("/test", SubscriberScope(testHMACSecret), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Subscriber-Token", "no-dot-separator")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSubscriberScope_InvalidBase64(t *testing.T) {
	r, w := setupScopeTest()
	r.GET("/test", SubscriberScope(testHMACSecret), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Subscriber-Token", "not-valid-base64!!!.abcdef")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSubscriberScope_InvalidSignature(t *testing.T) {
	r, w := setupScopeTest()
	payload := `{"subscriberId":"usr_123","timestamp":1234567890}`
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	token := encoded + ".invalidsignature"

	r.GET("/test", SubscriberScope(testHMACSecret), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Subscriber-Token", token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSubscriberScope_ExpiredToken(t *testing.T) {
	r, w := setupScopeTest()
	expired := time.Now().Add(-25 * time.Hour).UnixMilli()
	token := generateToken("usr_123", expired)

	r.GET("/test", SubscriberScope(testHMACSecret), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Subscriber-Token", token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "expired")
}
