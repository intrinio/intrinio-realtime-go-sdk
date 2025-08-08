package composite

// GreekUpdateFrequency represents when Greeks should be recalculated
type GreekUpdateFrequency uint32

const (
	EveryOptionsTradeUpdate           GreekUpdateFrequency = 1 << iota
	EveryOptionsQuoteUpdate          GreekUpdateFrequency = 1 << iota
	EveryRiskFreeInterestRateUpdate GreekUpdateFrequency = 1 << iota
	EveryDividendYieldUpdate        GreekUpdateFrequency = 1 << iota
	EveryEquityTradeUpdate          GreekUpdateFrequency = 1 << iota
	EveryEquityQuoteUpdate          GreekUpdateFrequency = 1 << iota
)

// Has checks if the frequency contains the given flag
func (f GreekUpdateFrequency) Has(flag GreekUpdateFrequency) bool {
	return f&flag != 0
}

// Add adds the given flag to the frequency
func (f *GreekUpdateFrequency) Add(flag GreekUpdateFrequency) {
	*f |= flag
}

// Remove removes the given flag from the frequency
func (f *GreekUpdateFrequency) Remove(flag GreekUpdateFrequency) {
	*f &^= flag
} 