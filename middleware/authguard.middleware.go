package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ClientPageAuthGuard(isAuthenticated func(*gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isAuthenticated(c) {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func ClientAPIAuthGuard(isAuthenticated func(*gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isAuthenticated(c) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Unauthorized",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
