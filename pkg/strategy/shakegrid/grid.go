package shakegrid

import "github.com/c9s/bbgo/pkg/fixedpoint"

type Grid struct {
	UpperPrice fixedpoint.Value `json:"upperPrice"`
	LowerPrice fixedpoint.Value `json:"lowerPrice"`

	// Size is the spread of each grid
	Size         fixedpoint.Value `json:"size"`

	// Density is the number of total grids
	Density      fixedpoint.Value `json:"density"`

	// Pins are the pinned grid prices, from low to high
	Pins []fixedpoint.Value `json:"pins"`
}

func NewGrid(lower, upper, density fixedpoint.Value) *Grid {
	var height = upper - lower
	var size = height.Div(density)
	var pins []fixedpoint.Value

	for p := lower ; p <= upper ; p += size {
		pins = append(pins, p)
	}

	return &Grid{
		UpperPrice: upper,
		LowerPrice: lower,
		Density: density,
		Size: size,
		Pins: pins,
	}
}

func (g *Grid) ExtendUpperPrice(price fixedpoint.Value) {

}

func (g *Grid) ExtendLowerPrice(price fixedpoint.Value) {

}
