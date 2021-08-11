package currencie

// Pair constructs a symbol
type Pair struct {
	Name         string
	Enabled      bool
	Quote        string
	QuoteDecimal int
	Base         string
	BaseDecimal  int
	FilterPrice  float64
	FilterSize   float64
}

// Pair returns a currency pair string
func (p *Pair) Pair() string {
	return p.Base + p.Quote
}
