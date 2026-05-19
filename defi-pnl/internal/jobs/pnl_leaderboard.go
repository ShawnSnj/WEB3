package jobs

import (
	"fmt"
	"log"
	"time"

	"defi-pnl/internal/storage"
)

// RunPnLLeaderboardTop10 computes the top 10 wallets+tokens by a confidence-weighted
// PnL score across the last 7 fully-ingested calendar days (using the trade_flows
// view) and inserts them into pnl_leaderboard with stat_date = CURRENT_DATE.
//
// The window is `[CURRENT_DATE - 7d, CURRENT_DATE)` — i.e. the same set of complete
// days that RunTradesBackfill7Days ingests (today-7 … yesterday; today is excluded
// because subgraph data for today is still in progress). Using a calendar-aligned
// window instead of `NOW() - INTERVAL '7 days'` makes the ranking deterministic
// for the whole day: re-running the job at any hour produces the same output.
//
// The score is `pnl * LOG(1 + trade_count) * LEAST(roi, 3)`:
//   - pnl rewards absolute USD profit
//   - LOG(1 + trade_count) discounts one-shot lucky trades vs. repeated activity
//   - LEAST(roi, 3) caps ROI at 300% to keep tiny-buy / huge-sell outliers from
//     dominating the ranking
//
// The DELETE + INSERT runs inside one transaction so re-running the job for the
// same calendar day (e.g. startup trigger + 02:00 scheduled trigger) is idempotent.
func RunPnLLeaderboardTop10() error {
	tx, err := storage.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM pnl_leaderboard WHERE stat_date = CURRENT_DATE`); err != nil {
		return fmt.Errorf("delete today: %w", err)
	}

	res, err := tx.Exec(`
        INSERT INTO pnl_leaderboard (stat_date, wallet, token, roi, pnl, trade_count, score)
        WITH filtered AS (
            SELECT *
            FROM trade_flows
            WHERE timestamp >= CURRENT_DATE - INTERVAL '7 days'
              AND timestamp <  CURRENT_DATE
              AND side IN ('BUY', 'SELL')
        ),
        stable_tokens AS (
            SELECT UNNEST(ARRAY[
                'USDC','USDT','DAI',
                'EUROC',
                'USDe','sUSDe',
                'RLUSD','AUSD',
                'aEthUSDC'
            ]) AS token
        ),
        agg AS (
            SELECT
                wallet,
                CASE
                    WHEN side = 'BUY'  THEN token_out
                    WHEN side = 'SELL' THEN token_in
                END AS token,
                SUM(CASE WHEN side = 'BUY'  THEN amount_usd ELSE 0 END) AS buy_usd,
                SUM(CASE WHEN side = 'SELL' THEN amount_usd ELSE 0 END) AS sell_usd,
                COUNT(*) AS trade_count
            FROM filtered
            WHERE
                (token_in IN (SELECT token FROM stable_tokens)
                 OR token_out IN (SELECT token FROM stable_tokens))
            GROUP BY wallet, token
        ),
        scored AS (
            SELECT
                wallet,
                token,
                buy_usd,
                sell_usd,
                trade_count,
                (sell_usd - buy_usd)::numeric                            AS pnl,
                ((sell_usd - buy_usd) / NULLIF(buy_usd, 0))::numeric     AS roi
            FROM agg
        ),
        ranked AS (
            SELECT
                wallet,
                token,
                buy_usd,
                sell_usd,
                trade_count,
                pnl,
                roi,
                pnl * LOG((1 + trade_count)::numeric) * LEAST(roi, 3) AS score
            FROM scored
        )
        SELECT
            CURRENT_DATE,
            wallet,
            token,
            ROUND(roi,   4) AS roi,
            ROUND(pnl,   2) AS pnl,
            trade_count,
            ROUND(score, 2) AS score
        FROM ranked
        WHERE
            token NOT IN (SELECT token FROM stable_tokens)
            AND buy_usd > 50000
            AND trade_count >= 5
            AND roi >0.1
        ORDER BY score DESC
        LIMIT 10
    `)
	if err != nil {
		return fmt.Errorf("insert pnl_leaderboard: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	n, _ := res.RowsAffected()
	log.Printf("pnl: inserted %d top-10 rows for %s", n, time.Now().Format("2006-01-02"))
	return nil
}
