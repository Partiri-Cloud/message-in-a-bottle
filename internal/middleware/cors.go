package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS enables cross-origin browser access to the API. The SDK runs in the
// browser on a different origin than this API, so preflight (OPTIONS) requests
// must be answered with the appropriate Access-Control-* headers; without them
// the browser blocks the request and gin returns 404 for the unrouted OPTIONS.
//
// allowedOrigins is an explicit allowlist (from CORS_ALLOWED_ORIGINS). A single
// entry of "*" allows any origin. When the request Origin is not allowed, no
// CORS headers are added and a preflight still short-circuits with 204 — the
// browser then blocks the real request, which is the intended fail-closed
// behaviour.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowAll := false
	set := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
		}
		set[o] = struct{}{}
	}

	return func(c *gin.Context) {
		// Vary unconditionally — on the reject path and on requests with no Origin
		// at all. A response cached from an Origin-less request would otherwise
		// carry no Vary, so a shared cache would not key on Origin and could serve
		// that header-less copy to a browser on an allowed origin, which then
		// blocks it for want of Access-Control-Allow-Origin.
		c.Header("Vary", "Origin")

		if origin := c.GetHeader("Origin"); origin != "" {
			_, ok := set[origin]
			if allowAll || ok {
				if allowAll {
					c.Header("Access-Control-Allow-Origin", "*")
				} else {
					c.Header("Access-Control-Allow-Origin", origin)
				}
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Subscriber-Token, X-Request-Id")
				c.Header("Access-Control-Max-Age", "86400")
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
