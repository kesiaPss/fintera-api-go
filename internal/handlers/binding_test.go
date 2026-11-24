package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestBindNestedOrFlat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		key         string
		body        string
		expected    TestStruct
		expectError bool
	}{
		{
			name:        "Nested Structure",
			key:         "data",
			body:        `{"data": {"name": "Alice", "age": 30}}`,
			expected:    TestStruct{Name: "Alice", Age: 30},
			expectError: false,
		},
		{
			name:        "Flat Structure",
			key:         "data",
			body:        `{"name": "Bob", "age": 25}`,
			expected:    TestStruct{Name: "Bob", Age: 25},
			expectError: false,
		},
		{
			name:        "Nested Structure with Missing Key Fallback",
			key:         "data",
			body:        `{"other": "value", "name": "Charlie", "age": 40}`,
			expected:    TestStruct{Name: "Charlie", Age: 40},
			expectError: false,
		},
		{
			name:        "Nested Structure with Different Key",
			key:         "project",
			body:        `{"project": {"name": "David", "age": 35}}`,
			expected:    TestStruct{Name: "David", Age: 35},
			expectError: false,
		},
		{
			name:        "Invalid JSON",
			key:         "data",
			body:        `{"name": "Eve", "age": "invalid"}`, // age is int
			expected:    TestStruct{},
			expectError: true,
		},
		{
			name:        "Nested but Invalid Content",
			key:         "data",
			body:        `{"data": {"name": "Frank", "age": "invalid"}}`,
			expected:    TestStruct{},
			expectError: true,
		},
		{
			name:        "Nested Key Present but Invalid Type",
			key:         "data",
			body:        `{"data": "some string"}`,
			expected:    TestStruct{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			var result TestStruct
			err := BindNestedOrFlat(c, tt.key, &result)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestBindNestedOrFlatOversizedBodyAnswers413 verifies the BodyLimit
// middleware's MaxBytesReader error surfaces as ErrBodyTooLarge and
// WriteBindError answers 413 with the Spanish message.
func TestBindNestedOrFlatOversizedBodyAnswers413(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"name": "` + strings.Repeat("a", 2048) + `"}`
	c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	// Simulate what middleware.BodyLimit installs.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 100)

	var result TestStruct
	err := BindNestedOrFlat(c, "data", &result)
	require.ErrorIs(t, err, ErrBodyTooLarge)
	assert.True(t, IsBodyTooLarge(err))

	WriteBindError(c, err, "Datos inválidos: ")

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "excede el tamaño máximo permitido")
}

// TestWriteBindErrorKeeps400ForInvalidJSON verifies ordinary binding failures
// keep the handler's 400 message prefix.
func TestWriteBindErrorKeeps400ForInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"age": "invalid"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	var result TestStruct
	err := BindNestedOrFlat(c, "data", &result)
	require.Error(t, err)

	WriteBindError(c, err, "Datos inválidos: ")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Datos inválidos: ")
}
