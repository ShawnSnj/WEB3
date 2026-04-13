package storage

import (
	"database/sql"
	"time"

	"defi-pnl/internal/pnl"
)

func GetTransactions(address string) ([]pnl.Transaction, error) {
	rows, err := DB.Query(`
        SELECT is_buy, amount, price , protocol
        FROM transactions
        WHERE user_address = $1
        ORDER BY timestamp ASC
    `, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []pnl.Transaction
	for rows.Next() {
		var tx pnl.Transaction
		if err := rows.Scan(&tx.IsBuy, &tx.Amount, &tx.Price, &tx.Protocol); err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return txs, nil
}

// LeaderboardEntry is one row for daily_leaderboard (tx_type U = origin aggregate, B = sender aggregate).
type LeaderboardEntry struct {
	TxAddress string
	TxType    string // "U" or "B"
	Volume    float64
}

func DailyLeaderboardExists(date time.Time) (bool, error) {
	var exists bool
	err := DB.QueryRow(`
        SELECT EXISTS(
            SELECT 1
            FROM daily_leaderboard
            WHERE date = $1
        )
    `, date).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func InsertDailyLeaderboard(date time.Time, entries []LeaderboardEntry) error {
	for _, e := range entries {
		_, err := DB.Exec(`
            INSERT INTO daily_leaderboard (date, tx_address, volume, tx_type)
            VALUES ($1, $2, $3, $4)
            ON CONFLICT (date, tx_address, tx_type) DO NOTHING
        `, date, e.TxAddress, e.Volume, e.TxType)
		if err != nil {
			return err
		}
	}
	return nil
}

// LatestLeaderboardDate returns the newest calendar date that has leaderboard rows.
func LatestLeaderboardDate() (time.Time, bool, error) {
	var d sql.NullTime
	err := DB.QueryRow(`SELECT MAX(date) FROM daily_leaderboard`).Scan(&d)
	if err != nil {
		return time.Time{}, false, err
	}
	if !d.Valid {
		return time.Time{}, false, nil
	}
	return d.Time.UTC(), true, nil
}

// GetDailyLeaderboardTop returns up to limit rows for the calendar date and tx_type (U or B), highest volume first.
func GetDailyLeaderboardTop(date time.Time, txType string, limit int) ([]LeaderboardEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := DB.Query(`
        SELECT tx_address, volume, tx_type
        FROM daily_leaderboard
        WHERE date = $1::date AND tx_type = $2
        ORDER BY volume DESC
        LIMIT $3
    `, date, txType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.TxAddress, &e.Volume, &e.TxType); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AllTimeLeaderboardRow ranks one (tx_address, tx_type) by summed volume across all days.
type AllTimeLeaderboardRow struct {
	TxAddress   string
	TxType      string
	TotalVolume float64
	PeakDate    time.Time
}

func GetAllTimeLeaderboardTopByType(txType string, limit int) ([]AllTimeLeaderboardRow, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := DB.Query(`
        WITH totals AS (
            SELECT tx_address, tx_type, SUM(volume) AS total_volume
            FROM daily_leaderboard
            WHERE tx_type = $1
            GROUP BY tx_address, tx_type
        ),
        best AS (
            SELECT DISTINCT ON (tx_address, tx_type)
                tx_address,
                tx_type,
                date AS peak_date
            FROM daily_leaderboard
            WHERE tx_type = $1
            ORDER BY tx_address, tx_type, volume DESC, date DESC
        )
        SELECT t.tx_address, t.tx_type, t.total_volume, b.peak_date
        FROM totals t
        JOIN best b ON b.tx_address = t.tx_address AND b.tx_type = t.tx_type
        ORDER BY t.total_volume DESC
        LIMIT $2
    `, txType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AllTimeLeaderboardRow
	for rows.Next() {
		var r AllTimeLeaderboardRow
		if err := rows.Scan(&r.TxAddress, &r.TxType, &r.TotalVolume, &r.PeakDate); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
