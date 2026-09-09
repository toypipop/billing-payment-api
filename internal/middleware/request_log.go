package middleware

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"billing-payment-api/internal/response"
	"github.com/gin-gonic/gin"
)

// RequestLog records the final response status once, including recovered panics.
func RequestLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		requestID := rand.Text()
		c.Header(response.RequestIDHeader, requestID)
		var panicStack string
		abortConnection := false
		defer func() {
			if recovered := recover(); recovered != nil {
				panicStack = string(debug.Stack())
				cause := fmt.Errorf("panic: %v", recovered)
				if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					abortConnection = true
				}
				if !c.Writer.Written() && !abortConnection {
					response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", cause)
				} else {
					// Once headers are sent, preserve the actual HTTP status in the
					// log and do not append an error body to a partial response.
					_ = c.Error(cause)
					c.Abort()
					abortConnection = true
				}
			}
			status := c.Writer.Status()
			if abortConnection && !c.Writer.Written() {
				status = 0 // No HTTP response was sent.
			}
			level := slog.LevelInfo
			if status >= 500 || panicStack != "" {
				level = slog.LevelError
			} else if status >= 400 {
				level = slog.LevelWarn
			}
			attrs := []slog.Attr{
				slog.String("request_id", requestID),
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.String("route", c.FullPath()),
				slog.Int("status", status),
				slog.String("status_text", http.StatusText(status)),
				slog.Float64("latency_ms", float64(time.Since(started).Microseconds())/1000),
				slog.Int("response_bytes", max(c.Writer.Size(), 0)),
			}
			if code := c.GetString("error_code"); code != "" {
				attrs = append(attrs, slog.String("error_code", code), slog.String("error", c.GetString("error_message")))
			}
			if len(c.Errors) > 0 {
				attrs = append(attrs, slog.Any("causes", c.Errors.Errors()))
			}
			if panicStack != "" {
				attrs = append(attrs, slog.String("stack", panicStack))
			}
			if abortConnection {
				attrs = append(attrs, slog.Bool("response_aborted", true))
			}
			logger.LogAttrs(c.Request.Context(), level, "http_request", attrs...)
			if abortConnection {
				// Let net/http close the connection (or reset the HTTP/2 stream)
				// instead of treating a truncated response as a completed success.
				panic(http.ErrAbortHandler)
			}
		}()
		c.Next()
	}
}
