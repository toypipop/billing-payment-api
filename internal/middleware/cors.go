package middleware

import "github.com/gin-gonic/gin"

// CORS permits the local Swagger UI container to call the API from a browser.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Origin") == "http://localhost:8081" {
			c.Header("Access-Control-Allow-Origin", "http://localhost:8081")
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
