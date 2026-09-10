package middleware

import (
	"context"
	"github.com/gin-gonic/gin"
	"time"
)

// Deadline cancels DB queries and pool waits through the request context.
// Execute synchronously: a timeout must not leave a background handler writing.
func Deadline(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
