package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PushedSignal mirrors a row in pushed_signals. CreatedAt is the alert job's
// run-start timestamp; all rows inserted in the same run share the same
// CreatedAt so the per-wallet watermark advances cleanly.
type PushedSignal struct {
	TxHash     string
	LogIndex   int64
	Wallet     string
	Token0     string
	Token1     string
	Side       string
	Amount0In  float64
	Amount0Out float64
	Amount1In  float64
	Amount1Out float64
	AmountUSD  float64
	Timestamp  time.Time
	CreatedAt  time.Time
}

// LastPushedSignalCreatedAt returns the most recent created_at for a wallet's
// pushed_signals rows. The bool is false when the wallet has no rows yet,
// in which case the caller should fall back to a sensible default (e.g.
// runStart - 1h).
func LastPushedSignalCreatedAt(wallet string) (time.Time, bool, error) {
	var t sql.NullTime
	err := DB.QueryRow(`
        SELECT MAX(created_at)
        FROM pushed_signals
        WHERE wallet = $1
    `, wallet).Scan(&t)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("max created_at: %w", err)
	}
	if !t.Valid {
		return time.Time{}, false, nil
	}
	return t.Time, true, nil
}

// InsertPushedSignals inserts each candidate row and returns the subset that
// were actually written (i.e. didn't collide on the (tx_hash, log_index)
// primary key). The caller should send Telegram alerts only for the returned
// rows so a retried job never re-notifies subscribers about the same swap.
func InsertPushedSignals(rows []PushedSignal) ([]PushedSignal, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	tx, err := DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
        INSERT INTO pushed_signals (
            tx_hash, log_index, wallet,
            token0, token1, side,
            amount0_in, amount0_out, amount1_in, amount1_out,
            amount_usd, timestamp, created_at
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
        ON CONFLICT (tx_hash, log_index) DO NOTHING
        RETURNING tx_hash, log_index
    `)
	if err != nil {
		return nil, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	inserted := make([]PushedSignal, 0, len(rows))
	for _, r := range rows {
		var th string
		var li int64
		err := stmt.QueryRow(
			r.TxHash, r.LogIndex, r.Wallet,
			r.Token0, r.Token1, r.Side,
			r.Amount0In, r.Amount0Out, r.Amount1In, r.Amount1Out,
			r.AmountUSD, r.Timestamp, r.CreatedAt,
		).Scan(&th, &li)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return inserted, fmt.Errorf("insert %s#%d: %w", r.TxHash, r.LogIndex, err)
		}
		inserted = append(inserted, r)
	}
	if err := tx.Commit(); err != nil {
		return inserted, fmt.Errorf("commit: %w", err)
	}
	return inserted, nil
}
