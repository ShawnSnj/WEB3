package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"defi-pnl/internal/storage"
)

// baseTokens are treated as "money" for side classification.
var baseTokens = map[string]bool{
	"USDC":  true,
	"USDT":  true,
	"DAI":   true,
	"EUROC": true,
}

// tradesSwap is the Swap shape used for trades_v2 ingestion.
type tradesSwap struct {
	Transaction struct {
		ID string `json:"id"`
	} `json:"transaction"`
	LogIndex  string `json:"logIndex"`
	Origin    string `json:"origin"`
	Amount0   string `json:"amount0"`
	Amount1   string `json:"amount1"`
	AmountUSD string `json:"amountUSD"`
	Timestamp string `json:"timestamp"`
	Token0    struct {
		ID     string `json:"id"`
		Symbol string `json:"symbol"`
	} `json:"token0"`
	Token1 struct {
		ID     string `json:"id"`
		Symbol string `json:"symbol"`
	} `json:"token1"`
}

type tradesGraphResp struct {
	Data struct {
		Swaps []tradesSwap `json:"swaps"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// RunTradesBackfill7Days ingests the latest 7 completed days (now-7 → yesterday, inclusive),
// skipping any day whose rows already exist in trades_v2.
//
// With now = today at 02:00, this covers [today-7d, today-1d], i.e. yesterday is the most
// recent day processed. (today itself is excluded because it is still in progress.)
func RunTradesBackfill7Days() {
	now := time.Now()
	// Start from 7 days before today's local midnight; the loop then walks forward
	// through `today-7d, today-6d, ..., today-1d` (yesterday).
	startDay := calendarMidnight(now).AddDate(0, 0, -7)
	for i := 0; i < 7; i++ {
		RunTradesForDay(startDay.AddDate(0, 0, i))
	}
}

// RunTradesForDay fetches and inserts trades for one local calendar day (midnight → +24h).
func RunTradesForDay(dayStart time.Time) {
	dayLabel := dayStart.Format("2006-01-02")
	dayEnd := dayStart.Add(24 * time.Hour)

	exists, err := storage.TradesExistForDate(dayStart)
	if err != nil {
		log.Printf("trades: check existence for %s: %v", dayLabel, err)
		return
	}
	if exists {
		log.Printf("trades: skip %s (already exists)", dayLabel)
		return
	}

	log.Printf("trades: fetching %s", dayLabel)
	inserted, err := fetchAndInsertTradesForRange(dayStart, dayEnd)
	if err != nil {
		log.Printf("trades: fetch %s: %v", dayLabel, err)
		return
	}
	log.Printf("trades: inserted %d rows for %s", inserted, dayLabel)
}

func fetchAndInsertTradesForRange(dayStart, dayEnd time.Time) (int, error) {
	endpoint := graphEndpoint()
	if endpoint == "" {
		return 0, fmt.Errorf("graph endpoint not configured (set GRAPH_API_KEY or GRAPH_ENDPOINT)")
	}

	from := dayStart.Unix()
	to := dayEnd.Unix()
	cursor := from
	total := 0

	for {
		page, err := fetchTradesPage(endpoint, cursor, to)
		if err != nil {
			return total, err
		}
		if len(page) == 0 {
			break
		}

		batch := make([]storage.Trade, 0, len(page))
		maxTS := cursor
		for _, s := range page {
			ts := parseInt(s.Timestamp)
			if ts > maxTS {
				maxTS = ts
			}
			batch = append(batch, swapToTrade(s))
		}
		if err := storage.InsertTrades(batch); err != nil {
			return total, fmt.Errorf("insert trades: %w", err)
		}
		total += len(batch)

		// Advance cursor past the last seen second to avoid re-fetching the same timestamp bucket.
		// Duplicates that share the same second across pages are guarded by the (tx_hash, log_index) PK.
		if maxTS <= cursor {
			break
		}
		cursor = maxTS
	}
	return total, nil
}

func swapToTrade(s tradesSwap) storage.Trade {
	amt0 := parseFloat(s.Amount0)
	amt1 := parseFloat(s.Amount1)

	// amount0/amount1 in the Swap entity are signed pool-side deltas.
	// Positive → token flowed INTO the pool (user sold that token).
	// Negative → token flowed OUT of the pool (user received that token).
	var a0In, a0Out, a1In, a1Out float64
	if amt0 > 0 {
		a0In = amt0
	} else if amt0 < 0 {
		a0Out = -amt0
	}
	if amt1 > 0 {
		a1In = amt1
	} else if amt1 < 0 {
		a1Out = -amt1
	}

	return storage.Trade{
		TxHash:     s.Transaction.ID,
		LogIndex:   parseInt(s.LogIndex),
		Wallet:     s.Origin,
		Token0:     tokenLabel(s.Token0.Symbol, s.Token0.ID),
		Token1:     tokenLabel(s.Token1.Symbol, s.Token1.ID),
		Amount0In:  a0In,
		Amount0Out: a0Out,
		Amount1In:  a1In,
		Amount1Out: a1Out,
		AmountUSD:  parseFloat(s.AmountUSD),
		Timestamp:  time.Unix(parseInt(s.Timestamp), 0).UTC(),
	}
}

// BackfillTradesSide classifies every trades_v2 row whose `side` is still NULL,
// using token-direction (amount0_in/out) and the base-token whitelist.
func BackfillTradesSide() error {
	db := storage.DB
	rows, err := db.Query(`
        SELECT tx_hash, log_index, token0, token1,
               amount0_in, amount0_out, amount1_in, amount1_out
        FROM trades_v2
        WHERE side IS NULL
    `)
	if err != nil {
		return fmt.Errorf("select null-side rows: %w", err)
	}
	defer rows.Close()

	updateStmt, err := db.Prepare(`
        UPDATE trades_v2
        SET side = $1
        WHERE tx_hash = $2 AND log_index = $3
    `)
	if err != nil {
		return fmt.Errorf("prepare update: %w", err)
	}
	defer updateStmt.Close()

	count := 0
	for rows.Next() {
		var t storage.Trade
		if err := rows.Scan(
			&t.TxHash, &t.LogIndex,
			&t.Token0, &t.Token1,
			&t.Amount0In, &t.Amount0Out,
			&t.Amount1In, &t.Amount1Out,
		); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		tokenIn, tokenOut := resolveDirection(t)
		side := classifySide(tokenIn, tokenOut)

		if _, err := updateStmt.Exec(side, t.TxHash, t.LogIndex); err != nil {
			log.Printf("trades-side: update %s#%d: %v", t.TxHash, t.LogIndex, err)
			continue
		}
		count++
		if count%1000 == 0 {
			log.Printf("trades-side: processed %d", count)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iter: %w", err)
	}
	log.Printf("trades-side: done, updated %d rows", count)
	return nil
}

func resolveDirection(t storage.Trade) (string, string) {
	if t.Amount0In > 0 {
		return t.Token0, t.Token1
	}
	return t.Token1, t.Token0
}

func classifySide(tokenIn, tokenOut string) string {
	if isBaseToken(tokenIn) && !isBaseToken(tokenOut) {
		return "BUY"
	}
	if isBaseToken(tokenOut) && !isBaseToken(tokenIn) {
		return "SELL"
	}
	return "SWAP"
}

func isBaseToken(token string) bool {
	return baseTokens[token]
}

func tokenLabel(symbol, id string) string {
	if symbol != "" {
		return symbol
	}
	return id
}

func fetchTradesPage(endpoint string, from, to int64) ([]tradesSwap, error) {
	query := fmt.Sprintf(`
    {
      swaps(
        first: 1000,
        orderBy: timestamp,
        orderDirection: asc,
        where: {
          amountUSD_gt: "1000",
          timestamp_gt: %d,
          timestamp_lt: %d
        }
      ) {
        transaction { id }
        logIndex
        origin
        amount0
        amount1
        amountUSD
        timestamp
        token0 { id symbol }
        token1 { id symbol }
      }
    }`, from, to)

	body, _ := json.Marshal(map[string]string{"query": query})
	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("graph POST: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("graph status %d: %s", resp.StatusCode, string(raw))
	}
	var out tradesGraphResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("graphql: %s", out.Errors[0].Message)
	}
	return out.Data.Swaps, nil
}

// StartDailyTradesScheduler runs the trades job once at startup, then every day at `hour` in `loc`.
// Each run does the 7-day fetch followed by a side-classification pass.
func StartDailyTradesScheduler(hour int, loc *time.Location) {
	if loc == nil {
		loc = time.Local
	}
	if hour < 0 || hour > 23 {
		hour = 2
	}
	go func() {
		//runTradesJobOnce("startup")
		for {
			d := durationUntilNextClock(hour, 0, loc)
			log.Printf("trades: next run in %v (at %02d:00 %s)", d, hour, loc.String())
			time.Sleep(d)
			runTradesJobOnce("scheduled")
		}
	}()
}

func runTradesJobOnce(reason string) {
	log.Printf("trades: running 7-day backfill (%s)", reason)
	RunTradesBackfill7Days()
	if err := BackfillTradesSide(); err != nil {
		log.Printf("trades-side: backfill error: %v", err)
	}
	if err := RunPnLLeaderboardTop10(); err != nil {
		log.Printf("pnl: leaderboard error: %v", err)
	}
}
