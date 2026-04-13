package pnl

import "sort"

type ProtocolPnL struct {
	Protocol   string  `json:"protocol"`
	Total      float64 `json:"total"`
	Realized   float64 `json:"realized"`
	Unrealized float64 `json:"unrealized"`
}

type Result struct {
	Total      float64       `json:"total"`
	Realized   float64       `json:"realized"`
	Unrealized float64       `json:"unrealized"`
	ByProtocol []ProtocolPnL `json:"by_protocol"`
}

func Calculate(txs []Transaction, currentPrice float64) Result {

	// 👉 每個 protocol 一個 position
	type Position struct {
		Amount  float64
		AvgCost float64
	}

	positions := make(map[string]*Position)
	realizedByProtocol := make(map[string]float64)

	// --- 計算 ---
	for _, tx := range txs {

		pos, ok := positions[tx.Protocol]
		if !ok {
			pos = &Position{}
			positions[tx.Protocol] = pos
		}

		// 買
		if tx.IsBuy {
			totalCost := pos.AvgCost*pos.Amount + tx.Price*tx.Amount
			pos.Amount += tx.Amount
			pos.AvgCost = totalCost / pos.Amount
		}

		// 賣
		if !tx.IsBuy {
			pnl := (tx.Price - pos.AvgCost) * tx.Amount
			realizedByProtocol[tx.Protocol] += pnl
			pos.Amount -= tx.Amount
		}
	}

	// --- 計算 unrealized ---
	unrealizedByProtocol := make(map[string]float64)

	for protocol, pos := range positions {
		unrealized := (currentPrice - pos.AvgCost) * pos.Amount
		unrealizedByProtocol[protocol] = unrealized
	}

	protocols := make([]string, 0, len(positions))
	for p := range positions {
		protocols = append(protocols, p)
	}
	sort.Strings(protocols)

	var total, realizedTotal, unrealizedTotal float64
	byProtocol := make([]ProtocolPnL, 0, len(protocols))

	for _, protocol := range protocols {
		r := realizedByProtocol[protocol]
		u := unrealizedByProtocol[protocol]
		t := r + u
		byProtocol = append(byProtocol, ProtocolPnL{
			Protocol:   protocol,
			Total:      t,
			Realized:   r,
			Unrealized: u,
		})
		total += t
		realizedTotal += r
		unrealizedTotal += u
	}

	return Result{
		Total:      total,
		Realized:   realizedTotal,
		Unrealized: unrealizedTotal,
		ByProtocol: byProtocol,
	}
}
