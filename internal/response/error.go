package response

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"
)

const RequestIDHeader = "X-Request-ID"

// Error keeps the public message separate from the internal cause logged by
// middleware. Clients can use the stable code and request ID for troubleshooting.
func Error(c *gin.Context, status int, code, message string, cause error) {
	if status == http.StatusInternalServerError && cause != nil {
		var pgErr *pgconn.PgError
		if errors.Is(cause, context.DeadlineExceeded) || errors.Is(c.Request.Context().Err(), context.DeadlineExceeded) {
			status, code, message = http.StatusServiceUnavailable, "REQUEST_TIMEOUT", "request exceeded its time limit"
		} else if errors.As(cause, &pgErr) && (pgErr.Code == "55P03" || pgErr.Code == "57014") {
			status, code, message = http.StatusServiceUnavailable, "DATABASE_TIMEOUT", "database operation exceeded its time limit"
		}
	}
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
