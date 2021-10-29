package shakegrid

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewGrid(t *testing.T) {
	upper := fixedpoint.NewFromFloat(500.0)
	lower := fixedpoint.NewFromFloat(100.0)
	density := fixedpoint.NewFromFloat(100.0)
	grid := NewGrid(lower, upper, density)
	assert.Equal(t, upper, grid.UpperPrice)
	assert.Equal(t, lower, grid.LowerPrice)
	assert.Equal(t, fixedpoint.NewFromFloat(4), grid.Size)
	if assert.Len(t, grid.Pins, 101) {
		assert.Equal(t, fixedpoint.NewFromFloat(100.0), grid.Pins[0])
		assert.Equal(t, fixedpoint.NewFromFloat(500.0), grid.Pins[100])
	}
}
