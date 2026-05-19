package storage

import "fmt"

// PnLLeaderboardEntry is one row from pnl_leaderboard, projected for the
// signals broadcast. PnL is exposed as the floored integer USD value (matches
// the message format) and TradeCount is the per-row activity counter.
type PnLLeaderboardEntry struct {
	Wallet     string
	Token      string
	PnLFloor   int64
	TradeCount int64
}

// GetTopPnLLeaderboard returns the most recent leaderboard rows ordered by
// stat_date DESC, pnl DESC. With LIMIT == 10 and 10 rows per stat_date, this
// surfaces today's leaderboard; on a day where the job hasn't run yet it
// gracefully falls back to the most recent populated date.
func GetTopPnLLeaderboard(limit int) ([]PnLLeaderboardEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := DB.Query(`
        SELECT wallet, token, FLOOR(pnl)::bigint AS pnl_floor, trade_count
        FROM pnl_leaderboard
        ORDER BY stat_date DESC, pnl DESC
        LIMIT $1
    `, limit)
	if err != nil {
		return nil, fmt.Errorf("query pnl_leaderboard: %w", err)
	}
	defer rows.Close()

	var out []PnLLeaderboardEntry
	for rows.Next() {
		var e PnLLeaderboardEntry
		if err := rows.Scan(&e.Wallet, &e.Token, &e.PnLFloor, &e.TradeCount); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
