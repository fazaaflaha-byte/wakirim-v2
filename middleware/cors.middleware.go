package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	allowedOrigins := parseOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	allowedMethods := "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	allowedHeaders := "Origin, Content-Type, Accept, Authorization, X-Requested-With"
	exposedHeaders := "Content-Length, Content-Type"

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin != "" {
			if isOriginAllowed(origin, allowedOrigins) {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Vary", "Origin")
			}
			c.Header("Access-Control-Allow-Methods", allowedMethods)
			c.Header("Access-Control-Allow-Headers", allowedHeaders)
			c.Header("Access-Control-Expose-Headers", exposedHeaders)
			c.Header("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func parseOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{
			"http://localhost:8080",
			"http://127.0.0.1:8080",
			"http://192.168.1.76:8080",
		}
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		val := strings.TrimSpace(p)
		if val != "" {
			out = append(out, val)
		}
	}
	return out
}

func isOriginAllowed(origin string, allowed []string) bool {
	for _, item := range allowed {
		if item == "*" || strings.EqualFold(item, origin) {
			return true
		}
	}
	return false
}
