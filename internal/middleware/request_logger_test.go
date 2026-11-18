package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/pkg/logger"
	"github.com/stretchr/testify/require"
)

func captureRequestLog(t *testing.T, fn func()) map[string]any {
	t.Helper()
	t.Setenv("LOG_FORMAT", "")

	r, w, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdout
	os.Stdout = w
	logger.Setup("test")
	t.Cleanup(func() {
		os.Stdout = original
		logger.Setup("test")
	})

	fn()

	require.NoError(t, w.Close())
	os.Stdout = original
	logger.Setup("test")
	out, err := io.ReadAll(r)
	require.NoError(t, err)

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) == nil && entry["msg"] == "Incoming request" {
			return entry
		}
	}
	t.Fatalf("no Incoming request log in output: %s", out)
	return nil
}

func newRequestLoggerRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestLogger())
	router.GET("/api/v1/projects/:project_id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func TestRequestLoggerGeneratesRequestIDAndLogsRoute(t *testing.T) {
	router := newRequestLoggerRouter()
	var recorder *httptest.ResponseRecorder

	entry := captureRequestLog(t, func() {
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/projects/42?token=secret", nil))
	})

	requestID := recorder.Header().Get(RequestIDHeader)
	require.NotEmpty(t, requestID)
	require.Equal(t, requestID, entry["requestId"])
	require.Equal(t, http.MethodGet, entry["method"])
	require.Equal(t, "/api/v1/projects/:project_id", entry["route"])
	require.EqualValues(t, http.StatusOK, entry["status"])
	require.Contains(t, entry, "durationMs")
	require.NotContains(t, entry, "path", "raw path and query must not be logged")
}

func TestRequestLoggerPropagatesValidRequestID(t *testing.T) {
	router := newRequestLoggerRouter()
	var recorder *httptest.ResponseRecorder

	entry := captureRequestLog(t, func() {
		recorder = httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/42", nil)
		req.Header.Set(RequestIDHeader, "edge-abc-123")
		router.ServeHTTP(recorder, req)
	})

	require.Equal(t, "edge-abc-123", recorder.Header().Get(RequestIDHeader))
	require.Equal(t, "edge-abc-123", entry["requestId"])
}

func TestRequestLoggerReplacesInvalidRequestID(t *testing.T) {
	router := newRequestLoggerRouter()
	var recorder *httptest.ResponseRecorder

	entry := captureRequestLog(t, func() {
		recorder = httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/42", nil)
		req.Header.Set(RequestIDHeader, "bad id\nwith newline")
		router.ServeHTTP(recorder, req)
	})

	generated := recorder.Header().Get(RequestIDHeader)
	require.NotEmpty(t, generated)
	require.NotEqual(t, "bad id\nwith newline", generated)
	require.Equal(t, generated, entry["requestId"])
}

func TestRequestLoggerLabelsUnmatchedRoutes(t *testing.T) {
	router := newRequestLoggerRouter()

	entry := captureRequestLog(t, func() {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/nope", nil))
	})

	require.Equal(t, "unmatched", entry["route"])
	require.EqualValues(t, http.StatusNotFound, entry["status"])
}
