package pnl

type Transaction struct {
	IsBuy    bool
	Amount   float64
	Price    float64
	Protocol string
}
