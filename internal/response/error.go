package response

import "github.com/gin-gonic/gin"

const RequestIDHeader = "X-Request-ID"

// Error keeps the public message separate from the internal cause logged by
// middleware. Clients can use the stable code and request ID for troubleshooting.
func Error(c *gin.Context, status int, code, message string, cause error) {
	c.Set("error_code", code)
	c.Set("error_message", message)
	if cause != nil {
		_ = c.Error(cause)
	}
	c.AbortWithStatusJSON(status, gin.H{
		"error":      message,
		"code":       code,
		"request_id": c.Writer.Header().Get(RequestIDHeader),
	})
}
