package storage

import "time"

// Trade is one row in the `trades_v2` table. `side` is intentionally not part of this struct;
// it is computed in a separate backfill pass after the raw rows are stored.
type Trade struct {
	TxHash     string
	LogIndex   int64
	Wallet     string
	Token0     string
	Token1     string
	Amount0In  float64
	Amount0Out float64
	Amount1In  float64
	Amount1Out float64
	AmountUSD  float64
	Timestamp  time.Time
}

// TradesExistForDate reports whether trades_v2 has any row whose timestamp falls in the given local calendar day.
func TradesExistForDate(dayStart time.Time) (bool, error) {
	dayEnd := dayStart.Add(24 * time.Hour)
	var exists bool
	err := DB.QueryRow(`
        SELECT EXISTS(
            SELECT 1
            FROM trades_v2
            WHERE timestamp >= $1 AND timestamp < $2
        )
    `, dayStart, dayEnd).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// InsertTrades writes a batch of trades in a single transaction. Duplicate (tx_hash, log_index) rows are ignored.
func InsertTrades(trades []Trade) error {
	if len(trades) == 0 {
		return nil
	}
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
        INSERT INTO trades_v2 (
            tx_hash, log_index, wallet,
            token0, token1,
            amount0_in, amount0_out, amount1_in, amount1_out,
            amount_usd, timestamp
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
        ON CONFLICT (tx_hash, log_index) DO NOTHING
    `)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, t := range trades {
		if _, err := stmt.Exec(
			t.TxHash, t.LogIndex, t.Wallet,
			t.Token0, t.Token1,
			t.Amount0In, t.Amount0Out, t.Amount1In, t.Amount1Out,
			t.AmountUSD, t.Timestamp,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
