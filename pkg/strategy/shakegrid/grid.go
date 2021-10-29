package shakegrid

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Grid struct {
	UpperPrice fixedpoint.Value `json:"upperPrice"`
	LowerPrice fixedpoint.Value `json:"lowerPrice"`

	// Spread is the spread of each grid
	Spread fixedpoint.Value `json:"spread"`

	// Size is the number of total grids
	Size fixedpoint.Value `json:"size"`

	// Pins are the pinned grid prices, from low to high
	Pins []fixedpoint.Value `json:"pins"`
}

func NewGrid(lower, upper, density fixedpoint.Value) *Grid {
	var height = upper - lower
	var size = height.Div(density)
	var pins []fixedpoint.Value

	for p := lower; p <= upper; p += size {
		pins = append(pins, p)
	}

	return &Grid{
		UpperPrice: upper,
		LowerPrice: lower,
		Size:       density,
		Spread:     size,
		Pins:       pins,
	}
}

func (g *Grid) ExtendUpperPrice(upper fixedpoint.Value) (newPins []fixedpoint.Value) {
	g.UpperPrice = upper

	// since the grid is extended, the size should be updated as well
	g.Size = (g.UpperPrice - g.LowerPrice).Div(g.Spread).Floor()

	lastPin := g.Pins[ len(g.Pins) - 1 ]
	for p := lastPin + g.Spread; p <= g.UpperPrice; p += g.Spread {
		newPins = append(newPins, p)
	}

	g.Pins = append(g.Pins, newPins...)
	return newPins
}

func (g *Grid) ExtendLowerPrice(lower fixedpoint.Value) (newPins []fixedpoint.Value) {
	g.LowerPrice = lower

	// since the grid is extended, the size should be updated as well
	g.Size = (g.UpperPrice - g.LowerPrice).Div(g.Spread).Floor()

	firstPin := g.Pins[0]
	numToAdd := (firstPin - g.LowerPrice).Div(g.Spread).Floor()
	if numToAdd == 0 {
		return newPins
	}

	for p := firstPin - g.Spread.Mul(numToAdd); p < firstPin; p += g.Spread {
		newPins = append(newPins, p)
	}

	g.Pins = append(newPins, g.Pins...)
	return newPins
}
