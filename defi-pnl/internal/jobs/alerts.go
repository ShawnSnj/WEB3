package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"defi-pnl/internal/storage"
	"defi-pnl/internal/telegram"
)

const (
	alertJobInterval     = 10 * time.Minute
	alertWalletLimit     = 10
	alertSwapPageLimit   = 100
	alertLookbackCeiling = time.Hour
	alertsPerMessage     = 8
)

// alertMinUSD is the minimum amountUSD for a swap to be considered a "smart
// money" signal. Configurable via ALERT_MIN_USD; defaults to $1,000 to match
// the trades_v2 ingest threshold.
func alertMinUSD() int {
	if v := os.Getenv("ALERT_MIN_USD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 1000
}

// RunSmartMoneyAlerts fetches new swaps for the top-10 wallets in
// pnl_leaderboard, persists them to pushed_signals, and pushes a formatted
// alert message to every active subscriber. Idempotent across retries: only
// rows that actually got inserted (i.e. didn't collide on tx_hash+log_index)
// are messaged.
func RunSmartMoneyAlerts() error {
	runStart := time.Now()

	leaderboard, err := storage.GetTopPnLLeaderboard(alertWalletLimit)
	if err != nil {
		return fmt.Errorf("read leaderboard: %w", err)
	}
	if len(leaderboard) == 0 {
		log.Printf("alerts: pnl_leaderboard is empty, skipping")
		return nil
	}

	// Index leaderboard rows by lowercase wallet so the alert formatter can
	// look up rank / trade_count / realized pnl when rendering each wallet's
	// block. Subgraph "origin" comes back lowercase, so we normalize on insert.
	// Rank is the 1-indexed position from GetTopPnLLeaderboard's ORDER BY
	// (stat_date DESC, pnl DESC), so the first row is "Top 1".
	stats := make(map[string]walletStats, len(leaderboard))
	for i, top := range leaderboard {
		stats[strings.ToLower(top.Wallet)] = walletStats{
			Rank:  i + 1,
			Entry: top,
		}
	}

	var candidates []storage.PushedSignal
	for _, top := range leaderboard {
		swaps, err := fetchAlertSwapsForWallet(top.Wallet, since(top.Wallet, runStart))
		if err != nil {
			log.Printf("alerts: fetch wallet=%s: %v", top.Wallet, err)
			continue
		}
		for _, s := range swaps {
			candidates = append(candidates, swapToPushedSignal(s, runStart))
		}
	}
	if len(candidates) == 0 {
		log.Printf("alerts: no new swaps from top-%d wallets", len(leaderboard))
		return nil
	}

	inserted, err := storage.InsertPushedSignals(candidates)
	if err != nil {
		return fmt.Errorf("insert pushed_signals: %w", err)
	}
	if len(inserted) == 0 {
		log.Printf("alerts: %d candidate swaps but all were already pushed", len(candidates))
		return nil
	}

	chatIDs, err := storage.ListActiveSubscriberChatIDs()
	if err != nil {
		return fmt.Errorf("list active subscribers: %w", err)
	}
	if len(chatIDs) == 0 {
		log.Printf("alerts: %d new signals saved, no active subscribers to notify", len(inserted))
		return nil
	}

	log.Printf("alerts: broadcasting %d new signals to %d subscribers", len(inserted), len(chatIDs))
	for i := 0; i < len(inserted); i += alertsPerMessage {
		end := i + alertsPerMessage
		if end > len(inserted) {
			end = len(inserted)
		}
		msg := formatAlertsMessage(inserted[i:end], stats, runStart)
		for _, chatID := range chatIDs {
			telegram.SendMessage(chatID, msg)
			time.Sleep(signalSendDelay)
		}
	}
	return nil
}

// since picks the timestamp_gt watermark for a wallet's subgraph query. We
// take the later of (per-wallet last push, runStart - 1h) so the worst-case
// query window is bounded — even if the server has been down for a day, we
// only fetch the last hour of swaps per wallet.
func since(wallet string, runStart time.Time) int64 {
	floor := runStart.Add(-alertLookbackCeiling)
	watermark, ok, err := storage.LastPushedSignalCreatedAt(wallet)
	if err != nil {
		log.Printf("alerts: watermark wallet=%s: %v (using floor)", wallet, err)
		return floor.Unix()
	}
	if !ok || watermark.Before(floor) {
		return floor.Unix()
	}
	return watermark.Unix()
}

func fetchAlertSwapsForWallet(wallet string, sinceUnix int64) ([]tradesSwap, error) {
	endpoint := graphEndpoint()
	if endpoint == "" {
		return nil, fmt.Errorf("graph endpoint not configured")
	}
	query := fmt.Sprintf(`
    {
      swaps(
        first: %d,
        orderBy: timestamp,
        orderDirection: asc,
        where: {
          origin: "%s",
          amountUSD_gt: "%d",
          timestamp_gt: %d
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
    }`, alertSwapPageLimit, strings.ToLower(wallet), alertMinUSD(), sinceUnix)

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

func swapToPushedSignal(s tradesSwap, runStart time.Time) storage.PushedSignal {
	amt0 := parseFloat(s.Amount0)
	amt1 := parseFloat(s.Amount1)

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

	t0 := tokenLabel(s.Token0.Symbol, s.Token0.ID)
	t1 := tokenLabel(s.Token1.Symbol, s.Token1.ID)

	var tokenIn, tokenOut string
	if a0In > 0 {
		tokenIn, tokenOut = t0, t1
	} else {
		tokenIn, tokenOut = t1, t0
	}
	side := classifySide(tokenIn, tokenOut)

	return storage.PushedSignal{
		TxHash:     s.Transaction.ID,
		LogIndex:   parseInt(s.LogIndex),
		Wallet:     s.Origin,
		Token0:     t0,
		Token1:     t1,
		Side:       side,
		Amount0In:  a0In,
		Amount0Out: a0Out,
		Amount1In:  a1In,
		Amount1Out: a1Out,
		AmountUSD:  parseFloat(s.AmountUSD),
		Timestamp:  time.Unix(parseInt(s.Timestamp), 0).UTC(),
		CreatedAt:  runStart,
	}
}

// formatAlertsMessage renders the broadcast text. Signals are grouped by
// wallet (one block per wallet, in the order each wallet first appears in
// `signals`) so a single wallet that produced several new swaps in this run
// only takes one block — its newest swap becomes the "Latest move". Per-wallet
// trade_count and realized pnl come from the pre-fetched leaderboard, keyed
// by lowercase wallet to match the lowercased `origin` returned by the
// subgraph.
//
// `now` is currently unused but kept in the signature so callers can inject
// a clock without churn if we re-introduce a relative-time line later.
// walletStats bundles a wallet's leaderboard row with its 1-indexed rank for
// the broadcast formatter.
type walletStats struct {
	Rank  int
	Entry storage.PnLLeaderboardEntry
}

func formatAlertsMessage(signals []storage.PushedSignal, stats map[string]walletStats, now time.Time) string {
	_ = now

	type bucket struct {
		latest storage.PushedSignal
	}
	order := make([]string, 0)
	byWallet := make(map[string]bucket, len(signals))
	for _, s := range signals {
		key := strings.ToLower(s.Wallet)
		cur, ok := byWallet[key]
		if !ok {
			order = append(order, key)
			byWallet[key] = bucket{latest: s}
			continue
		}
		if s.Timestamp.After(cur.latest.Timestamp) {
			cur.latest = s
			byWallet[key] = cur
		}
	}

	var b strings.Builder
	b.WriteString("🚨 Smart Money Alert\n\n")
	for i, key := range order {
		if i > 0 {
			b.WriteString("\n")
		}
		s := byWallet[key].latest
		st := stats[key]
		// Rank may be 0 if the wallet somehow isn't in `stats` (shouldn't
		// happen on the live path — we only fetch swaps for leaderboard
		// wallets — but defensive against future refactors). In that case
		// we omit the "Top N " prefix rather than print "Top 0".
		walletLine := "Wallet " + s.Wallet
		if st.Rank > 0 {
			walletLine = fmt.Sprintf("Top %d Wallet %s", st.Rank, s.Wallet)
		}
		fmt.Fprintf(&b,
			"%s\n• %d trades\n• %s realized pnl\n• Latest move: %s %s (%s)\n",
			walletLine,
			st.Entry.TradeCount,
			formatUSDCompact(st.Entry.PnLFloor),
			s.Side,
			actionTokenForSignal(s),
			formatUSDCompact(int64(math.Floor(s.AmountUSD))),
		)
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatUSDCompact renders an integer dollar amount in short form: "$873",
// "$2.1k", "$17k", "$423k", "$1.2m", "$5b". One decimal is kept while the
// magnitude is below 10× the unit (so $2,062 → "$2.1k", not "$2k") and
// dropped above (so $17,xxx → "$17k", matching the requested style).
func formatUSDCompact(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	switch {
	case n >= 1_000_000_000:
		if n < 10_000_000_000 {
			return fmt.Sprintf("%s$%.1fb", sign, float64(n)/1_000_000_000)
		}
		return fmt.Sprintf("%s$%db", sign, n/1_000_000_000)
	case n >= 1_000_000:
		if n < 10_000_000 {
			return fmt.Sprintf("%s$%.1fm", sign, float64(n)/1_000_000)
		}
		return fmt.Sprintf("%s$%dm", sign, n/1_000_000)
	case n >= 1_000:
		if n < 10_000 {
			return fmt.Sprintf("%s$%.1fk", sign, float64(n)/1_000)
		}
		return fmt.Sprintf("%s$%dk", sign, n/1_000)
	default:
		return fmt.Sprintf("%s$%d", sign, n)
	}
}

// actionTokenForSignal picks the token most relevant to the signal's side:
// the asset bought (BUY → tokenOut), sold (SELL → tokenIn), or both for a
// non-base ↔ non-base SWAP.
func actionTokenForSignal(s storage.PushedSignal) string {
	var tokenIn, tokenOut string
	if s.Amount0In > 0 {
		tokenIn, tokenOut = s.Token0, s.Token1
	} else {
		tokenIn, tokenOut = s.Token1, s.Token0
	}
	switch s.Side {
	case "BUY":
		return tokenOut
	case "SELL":
		return tokenIn
	default:
		return tokenIn + "→" + tokenOut
	}
}

// StartAlertScheduler runs RunSmartMoneyAlerts once at startup and then every
// alertJobInterval (10 min). The lookback ceiling + (tx_hash, log_index)
// dedup make startup runs safe against double-notification, even after long
// downtime.
func StartAlertScheduler() {
	go func() {
		runAlertJobOnce("startup")
		ticker := time.NewTicker(alertJobInterval)
		defer ticker.Stop()
		for range ticker.C {
			runAlertJobOnce("scheduled")
		}
	}()
}

func runAlertJobOnce(reason string) {
	log.Printf("alerts: running (%s)", reason)
	if err := RunSmartMoneyAlerts(); err != nil {
		log.Printf("alerts: error: %v", err)
	}
}
