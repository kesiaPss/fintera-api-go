package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func post(router *gin.Engine, contentType string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestBodyLimitRejectsOversizedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyLimit(10))

	var readErr error
	router.POST("/echo", func(c *gin.Context) {
		_, readErr = io.ReadAll(c.Request.Body)
		c.Status(http.StatusOK)
	})

	w := post(router, "application/json", strings.Repeat("a", 100))
	assert.Equal(t, http.StatusOK, w.Code)
	require.Error(t, readErr, "body larger than the limit must fail to read")

	var maxBytesErr *http.MaxBytesError
	require.ErrorAs(t, readErr, &maxBytesErr, "the error must be *http.MaxBytesError")
}

func TestBodyLimitAllowsBodyWithinLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyLimit(1024))

	var readErr error
	var readBody string
	router.POST("/echo", func(c *gin.Context) {
		data, err := io.ReadAll(c.Request.Body)
		readErr, readBody = err, string(data)
		c.Status(http.StatusOK)
	})

	w := post(router, "application/json; charset=utf-8", `{"hello":"world"}`)
	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, readErr)
	assert.Equal(t, `{"hello":"world"}`, readBody)
}

func TestBodyLimitSkipsMultipartRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyLimit(10))

	var readErr error
	var total int
	router.POST("/echo", func(c *gin.Context) {
		data, err := io.ReadAll(c.Request.Body)
		readErr, total = err, len(data)
		c.Status(http.StatusOK)
	})

	boundary := "xyz"
	payload := strings.Repeat("content-", 100) // way over the 10 byte limit
	body := "--" + boundary + "\r\nContent-Disposition: form-data; name=\"f\"\r\n\r\n" +
		payload + "\r\n--" + boundary + "--\r\n"

	w := post(router, "multipart/form-data; boundary="+boundary, body)
	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, readErr, "multipart bodies are exempt from the JSON limit")
	assert.Equal(t, len(body), total)
}

func TestBodyLimitDisabledWhenNonPositive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyLimit(0))

	var readErr error
	router.POST("/echo", func(c *gin.Context) {
		_, readErr = io.ReadAll(c.Request.Body)
		c.Status(http.StatusOK)
	})

	w := post(router, "application/json", strings.Repeat("a", 1<<20))
	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, readErr)
}

// TestBodyLimitMakesShouldBindJSONFail documents the accepted behavior for
// routes bound via Gin: an oversized body surfaces as a binding error, so the
// handler's existing 400 path rejects it (413 is reserved for the direct-read
// sites in internal/handlers/binding.go).
func TestBodyLimitMakesShouldBindJSONFail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodyLimit(10))

	var bindErr error
	status := 0
	router.POST("/echo", func(c *gin.Context) {
		var obj map[string]string
		bindErr = c.ShouldBindJSON(&obj)
		if bindErr != nil {
			status = http.StatusBadRequest
		} else {
			status = http.StatusOK
		}
		c.Status(status)
	})

	w := post(router, "application/json", strings.Repeat("a", 100))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, http.StatusBadRequest, status)
	require.Error(t, bindErr)
}
