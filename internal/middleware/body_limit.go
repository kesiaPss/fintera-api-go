package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// BodyLimit caps the number of bytes read from a request body by wrapping it
// in http.MaxBytesReader. Oversized bodies make subsequent reads return an
// *http.MaxBytesError, which the binding helpers surface as HTTP 413.
//
// It is intentionally NOT applied to multipart/form-data requests: the upload
// endpoints (contract documents, drawings, photos, receipts, logo, avatar, CSV
// import) enforce their own, much larger, per-endpoint body and per-file
// limits via MaxBytesReader plus bounded ParseMultipartForm/c.FormFile memory.
//
// Register it innermost (on the /api/v1 group, after the global middleware) so
// it is the last wrapper before handlers: any global middleware that replaces
// the body runs first and the byte cap then applies to what handlers actually
// read.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 && !isMultipart(c) {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// isMultipart reports whether the request carries a multipart payload, which
// is exempt from the JSON body limit because upload endpoints apply their own
// larger limits.
func isMultipart(c *gin.Context) bool {
	return strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/")
}
