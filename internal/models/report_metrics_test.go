package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEffectiveness(t *testing.T) {
	t.Run("zero projection returns zero", func(t *testing.T) {
		assert.Equal(t, 0.0, Effectiveness(1500, 0))
		assert.Equal(t, 0.0, Effectiveness(0, 0))
	})

	t.Run("fully collected is 100", func(t *testing.T) {
		assert.Equal(t, 100.0, Effectiveness(1200, 1200))
	})

	t.Run("partial collection is a rounded percentage", func(t *testing.T) {
		assert.Equal(t, 50.0, Effectiveness(500, 1000))
		assert.Equal(t, 66.67, Effectiveness(2, 3))
	})

	t.Run("over collection exceeds 100", func(t *testing.T) {
		assert.Equal(t, 120.0, Effectiveness(1200, 1000))
	})
}
