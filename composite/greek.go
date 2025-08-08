package composite

import "math"

// Greek represents the calculated Greeks for an options contract
type Greek struct {
	ImpliedVolatility float64
	Delta             float64
	Gamma             float64
	Theta             float64
	Vega              float64
	IsValid           bool
}

// NewGreek creates a new Greek struct with the given values
func NewGreek(impliedVolatility float64, delta float64, gamma float64, theta float64, vega float64, isValid bool) Greek {
	return Greek{
		ImpliedVolatility: impliedVolatility,
		Delta:             delta,
		Gamma:             gamma,
		Theta:             theta,
		Vega:              vega,
		IsValid:           isValid,
	}
}

// IsValidGreek checks if the Greek values are valid (not NaN or infinite)
func (g Greek) IsValidGreek() bool {
	return g.IsValid && 
		!math.IsNaN(g.ImpliedVolatility) && !math.IsInf(g.ImpliedVolatility, 0) &&
		!math.IsNaN(g.Delta) && !math.IsInf(g.Delta, 0) &&
		!math.IsNaN(g.Gamma) && !math.IsInf(g.Gamma, 0) &&
		!math.IsNaN(g.Theta) && !math.IsInf(g.Theta, 0) &&
		!math.IsNaN(g.Vega) && !math.IsInf(g.Vega, 0)
} 