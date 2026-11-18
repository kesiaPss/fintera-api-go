package middleware

import (
	"log/slog"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sjperalta/fintera-api/pkg/logger"
)

const (
	RequestIDHeader = "X-Request-ID"
	RequestIDKey    = "requestID"
)

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// RequestLogger assigns each request an ID (propagating a valid X-Request-ID
// from the caller, otherwise generating one), returns it in the response
// header, and logs the request with its route template instead of the raw path
// so entries can be matched with proxy access logs.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		requestID := c.GetHeader(RequestIDHeader)
		if !validRequestID.MatchString(requestID) {
			requestID = uuid.NewString()
		}
		c.Set(RequestIDKey, requestID)
		c.Header(RequestIDHeader, requestID)

		c.Next()

		// Skip logging for health check to avoid noise
		if c.Request.URL.Path == "/api/v1/health" {
			return
		}

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		statusCode := c.Writer.Status()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		attrs := []any{
			slog.String("requestId", requestID),
			slog.String("method", c.Request.Method),
			slog.String("route", route),
			slog.Int("status", statusCode),
			slog.Float64("durationMs", float64(time.Since(start))/float64(time.Millisecond)),
			slog.String("ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
		}

		// Add error message if present
		if errorMessage != "" {
			attrs = append(attrs, slog.String("error", errorMessage))
		}

		// Add user ID if authenticated
		if userID, exists := c.Get("userID"); exists {
			attrs = append(attrs, slog.Any("user_id", userID))
		}

		msg := "Incoming request"
		if statusCode >= 500 {
			logger.Error(msg, attrs...)
		} else if statusCode >= 400 {
			logger.Warn(msg, attrs...)
		} else {
			logger.Info(msg, attrs...)
		}
	}
}
