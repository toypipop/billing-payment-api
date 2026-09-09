package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"billing-payment-api/internal/middleware"
	"billing-payment-api/internal/response"
	"github.com/gin-gonic/gin"
)

func TestRequestLogMatchesHTTPStatus(t *testing.T) {
	for _, status := range []int{200, 201, 204, 400, 404, 405, 409, 412, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var output bytes.Buffer
			r := gin.New()
			r.Use(middleware.RequestLog(slog.New(slog.NewJSONHandler(&output, nil))))
			r.POST("/test/:id", func(c *gin.Context) {
				if status >= 400 {
					var cause error
					if status >= 500 {
						cause = errors.New("private database failure")
					}
					response.Error(c, status, "TEST_ERROR", "public error", cause)
				} else if status == 204 {
					c.Status(status)
				} else {
					c.JSON(status, gin.H{"ok": true})
				}
			})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/test/123?token=secret-query", strings.NewReader(`{"secret":"secret-body"}`))
			req.Header.Set("Authorization", "Bearer secret-auth")
			req.Header.Set("Cookie", "session=secret-cookie")
			req.Header.Set("X-Request-ID", "untrusted-client-id")
			r.ServeHTTP(w, req)
			if w.Code != status {
				t.Fatalf("status = %d, want %d", w.Code, status)
			}
			if status == 204 && w.Body.Len() != 0 {
				t.Fatalf("204 has a body: %s", w.Body.String())
			}
			var log map[string]any
			if err := json.Unmarshal(output.Bytes(), &log); err != nil {
				t.Fatalf("expected one JSON log: %v: %s", err, output.String())
			}
			level := "INFO"
			if status >= 500 {
				level = "ERROR"
			} else if status >= 400 {
				level = "WARN"
			}
			if log["status"] != float64(status) || log["level"] != level || log["method"] != "POST" || log["path"] != "/test/123" || log["route"] != "/test/:id" || log["msg"] != "http_request" {
				t.Fatalf("unexpected log: %s", output.String())
			}
			id := w.Header().Get("X-Request-ID")
			if id == "" || id == "untrusted-client-id" || log["request_id"] != id {
				t.Fatalf("invalid request ID: %v", log)
			}
			if log["response_bytes"] != float64(w.Body.Len()) {
				t.Fatalf("incorrect size: %v", log)
			}
			if latency, ok := log["latency_ms"].(float64); !ok || latency < 0 {
				t.Fatalf("invalid latency: %v", log)
			}
			if status >= 400 {
				var body map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body["code"] != "TEST_ERROR" || body["error"] != "public error" || body["request_id"] != id || log["error_code"] != body["code"] {
					t.Fatalf("error contract: %v", body)
				}
			}
			if strings.Contains(w.Body.String(), "private database failure") {
				t.Fatal("internal error leaked to client")
			}
			if status >= 500 && !strings.Contains(output.String(), "private database failure") {
				t.Fatal("internal cause missing from server log")
			}
			for _, secret := range []string{"secret-query", "secret-body", "secret-auth", "secret-cookie"} {
				if strings.Contains(output.String(), secret) {
					t.Fatalf("log contains %s", secret)
				}
			}
		})
	}
}

func TestPanicRecoveryLog(t *testing.T) {
	var output bytes.Buffer
	r := gin.New()
	r.Use(middleware.RequestLog(slog.New(slog.NewJSONHandler(&output, nil))))
	r.GET("/panic", func(c *gin.Context) { panic("private panic detail") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != 500 || strings.Contains(w.Body.String(), "private panic detail") {
		t.Fatalf("panic response: %d %s", w.Code, w.Body.String())
	}
	var log map[string]any
	if err := json.Unmarshal(output.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	if log["status"] != float64(500) || log["level"] != "ERROR" || log["stack"] == "" || !strings.Contains(output.String(), "private panic detail") {
		t.Fatalf("panic log: %s", output.String())
	}
	// Recovery does not stop the server from handling subsequent requests.
	output.Reset()
	next := httptest.NewRecorder()
	r.ServeHTTP(next, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if next.Header().Get("X-Request-ID") == w.Header().Get("X-Request-ID") {
		t.Fatal("request IDs reused")
	}
}

func TestPanicAfterHeadersAbortsResponse(t *testing.T) {
	var output bytes.Buffer
	r := gin.New()
	r.Use(middleware.RequestLog(slog.New(slog.NewJSONHandler(&output, nil))))
	r.GET("/partial", func(c *gin.Context) {
		c.String(200, "partial")
		panic("write failed")
	})
	w := httptest.NewRecorder()
	func() {
		defer func() {
			if recovered := recover(); recovered != http.ErrAbortHandler {
				t.Fatalf("expected connection abort, got %v", recovered)
			}
		}()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/partial", nil))
	}()
	var log map[string]any
	if err := json.Unmarshal(output.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	if log["status"] != float64(200) || log["level"] != "ERROR" || log["response_aborted"] != true || w.Body.String() != "partial" {
		t.Fatalf("partial response log: %s", output.String())
	}
}
