package main

import (
	"math"
)

// OptimalDx implements the requested formula:
// dx = (sqrt(x*y*(1-fee)*price) - x) / (1-fee)
// Where:
//   x,y are reserves from the first pool (reserveX/reserveY)
//   price is the selling price of token Y in terms of token X (X per Y)
//   fee is the swap fee as a fraction (e.g. 0.003 for 0.3%)
func OptimalDx(x, y, price, fee float64) float64 {
	// Guard against invalid inputs producing NaN.
	if x <= 0 || y <= 0 || price <= 0 {
		return 0
	}
	oneMinusFee := 1.0 - fee
	if oneMinusFee <= 0 {
		return 0
	}
	return (math.Sqrt(x*y*oneMinusFee*price) - x) / oneMinusFee
}

// EstimateProfit simulates the two-leg swap using the same AMM math as Solidity SimpleDEX.
// It returns amountOut (final token X after both swaps) and profit = amountOut - amountIn (signed).
func EstimateProfit(
	reserveX1, reserveY1, reserveX2, reserveY2 uint64,
	amountIn uint64,
	feeBps uint64, // 30 for 0.3%
) (amountOut uint64, profit int64) {
	if amountIn == 0 || reserveX1 == 0 || reserveY1 == 0 || reserveX2 == 0 || reserveY2 == 0 {
		return 0, 0
	}
	// Leg 1: pool1 swap X -> Y.
	fee1 := (amountIn * feeBps) / 10000
	dxAfterFee := amountIn - fee1
	// dy = reserveY1 - k/(reserveX1 + dxAfterFee)
	k1 := reserveX1 * reserveY1
	newReserveX1 := reserveX1 + dxAfterFee
	if newReserveX1 == 0 {
		return 0, 0
	}
	newReserveY1 := k1 / newReserveX1
	if newReserveY1 >= reserveY1 {
		return 0, 0
	}
	dy := reserveY1 - newReserveY1
	if dy == 0 {
		return 0, 0
	}

	// Leg 2: pool2 swapReverse Y -> X.
	fee2 := (dy * feeBps) / 10000
	dyAfterFee := dy - fee2
	k2 := reserveX2 * reserveY2
	newReserveY2 := reserveY2 + dyAfterFee
	if newReserveY2 == 0 {
		return 0, 0
	}
	newReserveX2 := k2 / newReserveY2
	if newReserveX2 >= reserveX2 {
		return 0, 0
	}
	amountOut = reserveX2 - newReserveX2
	profit = int64(amountOut) - int64(amountIn)
	return amountOut, profit
}

