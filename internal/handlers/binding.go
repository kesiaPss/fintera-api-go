package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrBodyTooLarge is returned by BindNestedOrFlat when the BodyLimit
// middleware truncated an oversized request body.
var ErrBodyTooLarge = errors.New("request body too large")

// IsBodyTooLarge reports whether err comes from reading a body that exceeded
// the configured JSON request limit (http.MaxBytesError) or its handler-level
// sentinel.
func IsBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.Is(err, ErrBodyTooLarge) || errors.As(err, &maxBytesErr)
}

// WriteBindError responds to a BindNestedOrFlat failure: oversized bodies get
// 413 with a Spanish message, everything else gets 400 prefixed by the
// handler's own message.
func WriteBindError(c *gin.Context, err error, messagePrefix string) {
	if IsBodyTooLarge(err) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "El cuerpo de la solicitud excede el tamaño máximo permitido"})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": messagePrefix + err.Error()})
}

// BindNestedOrFlat attempts to bind the request body to obj.
// It first checks if the body contains a nested object with the given key (e.g. {"project": {...}}).
// If so, it binds that nested object to obj.
// If not, or if the key is missing, it attempts to bind the entire body to obj (e.g. {...}).
// This helper is used to support both nested and flat JSON structures for compatibility.
//
// When the BodyLimit middleware cuts off an oversized body, it returns
// ErrBodyTooLarge so callers can answer 413 via WriteBindError.
func BindNestedOrFlat(c *gin.Context, key string, obj interface{}) error {
	var bodyBytes []byte
	if c.Request.Body != nil {
		var readErr error
		bodyBytes, readErr = io.ReadAll(c.Request.Body)
		if readErr != nil {
			if IsBodyTooLarge(readErr) {
				return ErrBodyTooLarge
			}
			return readErr
		}
	}
	// Restore body for future binding or subsequent reads
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 1. Try Nested Structure { "key": { ... } }
	var nestedMap map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &nestedMap); err == nil {
		if val, ok := nestedMap[key]; ok {
			// Found the key, try to unmarshal the content of that key into obj
			// We return the result of this unmarshal directly.
			// If the nested object is invalid for the target struct, we return that error.
			return json.Unmarshal(val, obj)
		}
	}

	// 2. Fallback to Flat Structure { ... }
	// If the nested key was not found or the body wasn't a JSON object, try flat binding.
	return json.Unmarshal(bodyBytes, obj)
}
