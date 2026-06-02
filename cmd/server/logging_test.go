package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lsj/copylingo/internal/observability"
)

func TestRequestLoggingMiddlewareLogsSelectedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	restore := setTestLogger(&output)
	defer restore()

	router := gin.New()
	router.Use(requestLoggingMiddleware(), structuredRecoveryMiddleware())
	router.GET("/api/test", func(c *gin.Context) {
		if got := observability.InteractionID(c.Request.Context()); !strings.HasPrefix(got, "http-") {
			t.Fatalf("InteractionID() = %q, want http prefix", got)
		}
		c.Status(http.StatusCreated)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/test?secret=excluded", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if got := response.Header().Get("X-Request-ID"); !strings.HasPrefix(got, "http-") {
		t.Fatalf("X-Request-ID = %q, want http prefix", got)
	}
	entry := decodeServerLogEntry(t, output.Bytes())
	if got := entry["event"]; got != "http.completed" {
		t.Fatalf("entry[event] = %#v, want http.completed", got)
	}
	if got := entry["status"]; got != float64(http.StatusCreated) {
		t.Fatalf("entry[status] = %#v, want %d", got, http.StatusCreated)
	}
	if strings.Contains(output.String(), "secret") {
		t.Fatalf("access log leaked query string: %s", output.String())
	}
}

func TestRequestLoggingMiddlewareSuppressesHealthAndAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	restore := setTestLogger(&output)
	defer restore()

	router := gin.New()
	router.Use(requestLoggingMiddleware(), structuredRecoveryMiddleware())
	router.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/miniapp/handwriting/assets/app.js", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, path := range []string{"/health", "/miniapp/handwriting/assets/app.js"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if got := response.Header().Get("X-Request-ID"); !strings.HasPrefix(got, "http-") {
			t.Fatalf("%s X-Request-ID = %q, want http prefix", path, got)
		}
	}
	if output.Len() != 0 {
		t.Fatalf("suppressed routes produced logs: %s", output.String())
	}
}

func TestStructuredRecoveryMiddlewareLogsPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	restore := setTestLogger(&output)
	defer restore()

	router := gin.New()
	router.Use(requestLoggingMiddleware(), structuredRecoveryMiddleware())
	router.GET("/api/panic", func(c *gin.Context) { panic("boom") })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/panic", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(output.String(), `"event":"http.panic"`) {
		t.Fatalf("panic log missing: %s", output.String())
	}
	if !strings.Contains(output.String(), `"event":"http.completed"`) {
		t.Fatalf("completion log missing: %s", output.String())
	}
}

func setTestLogger(output *bytes.Buffer) func() {
	previous := slog.Default()
	slog.SetDefault(slog.New(observability.NewContextHandler(slog.NewJSONHandler(output, nil))))
	return func() { slog.SetDefault(previous) }
}

func decodeServerLogEntry(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal(body, &entry); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", body, err)
	}
	return entry
}
