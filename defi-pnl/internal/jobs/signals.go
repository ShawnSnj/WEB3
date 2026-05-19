package jobs

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"defi-pnl/internal/storage"
	"defi-pnl/internal/telegram"
)

// signalSendDelay paces outgoing Telegram messages to stay well under the Bot
// API's "30 messages/sec to different users" cap. With a small subscriber base
// this is essentially free; with thousands it prevents 429s.
const signalSendDelay = 50 * time.Millisecond

// RunSignalBroadcast pulls the latest top-10 from pnl_leaderboard and sends a
// formatted signals message to every currently-active subscriber.
//
// "Active" means: subscribers.status='active' AND start_at::date <= today
// AND expire_at::date >= today. A user whose subscription has expired or is
// still pending will not receive the broadcast.
//
// Failures sending to one chat (e.g. user blocked the bot, network blip) do
// not abort the broadcast — they are logged and the loop continues so other
// subscribers still get their message.
func RunSignalBroadcast() error {
	chatIDs, err := storage.ListActiveSubscriberChatIDs()
	if err != nil {
		return fmt.Errorf("list active subscribers: %w", err)
	}
	if len(chatIDs) == 0 {
		log.Printf("signals: no active subscribers, skipping broadcast")
		return nil
	}

	entries, err := storage.GetTopPnLLeaderboard(10)
	if err != nil {
		return fmt.Errorf("read leaderboard: %w", err)
	}
	if len(entries) == 0 {
		log.Printf("signals: pnl_leaderboard is empty, skipping broadcast")
		return nil
	}

	msg := formatSignalsMessage(entries, time.Now())
	log.Printf("signals: broadcasting top-%d leaderboard to %d subscribers", len(entries), len(chatIDs))

	for _, chatID := range chatIDs {
		telegram.SendMessage(chatID, msg)
		time.Sleep(signalSendDelay)
	}
	log.Printf("signals: broadcast done")
	return nil
}

// formatSignalsMessage builds the user-facing text. Kept pure (the clock is
// injected by the caller) so it's trivial to unit-test by passing a synthetic
// []PnLLeaderboardEntry plus a fixed time.
func formatSignalsMessage(entries []storage.PnLLeaderboardEntry, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🔥 Smart Money Signals — %s\n\n", now.Format("January 2, 2006"))
	b.WriteString("The latest 7 days top profitable wallets on Uniswap:\n\n")
	for i, e := range entries {
		fmt.Fprintf(&b,
			"top %d\nuser_address: %s\ntoken: %s\npnl: %s\ntrade_count: %d\n\n",
			i+1, e.Wallet, e.Token, formatUSD(e.PnLFloor), e.TradeCount,
		)
	}
	if footer := formatRotationFooter(entries); footer != "" {
		b.WriteString(footer)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatRotationFooter renders the "🐋 Smart money is rotating into …" hype
// block that highlights the 9th and 10th leaderboard wallets. Returns "" when
// the leaderboard is shorter than 10 rows or either of those rows has a
// non-positive realized PnL — the "+$X" / "smart money is rotating" framing
// only makes sense for two profitable wallets.
//
// The rotation token is taken from the 9th row; if the 10th row holds a
// different token the message still reads sensibly because the second wallet
// is introduced as "Another" without re-stating its token.
func formatRotationFooter(entries []storage.PnLLeaderboardEntry) string {
	if len(entries) < 10 {
		return ""
	}
	ninth := entries[8]
	tenth := entries[9]
	if ninth.PnLFloor <= 0 || tenth.PnLFloor <= 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "🐋 Smart money is rotating into %s again.\n\n", ninth.Token)
	fmt.Fprintf(&b,
		"One wallet made:\n+%s in the last 7 days\nacross just %d trades.\n\n",
		formatUSD(ninth.PnLFloor), ninth.TradeCount,
	)
	fmt.Fprintf(&b,
		"Another:\n+%s with %d trades.",
		formatUSD(tenth.PnLFloor), tenth.TradeCount,
	)
	return b.String()
}

// formatUSD renders an integer dollar amount as e.g. "$575,034" (with a
// leading "-" for negatives). Done by hand so we don't pull in
// golang.org/x/text just for thousands separators.
func formatUSD(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	head := len(s) % 3
	if head == 0 {
		head = 3
	}
	var b strings.Builder
	b.WriteString(s[:head])
	for i := head; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return sign + "$" + b.String()
}

// StartDailySignalScheduler fires RunSignalBroadcast once at startup and then
// every day at `hour:00` in `loc`.
//
// NOTE: every server restart will re-broadcast to all active subscribers.
// In production this is fine (restarts are rare); in development you can
// either temporarily set status='inactive' on your test subscribers, or
// gate the startup call behind an env flag if it becomes annoying.
func StartDailySignalScheduler(hour int, loc *time.Location) {
	if loc == nil {
		loc = time.Local
	}
	if hour < 0 || hour > 23 {
		hour = 3
	}
	go func() {
		runSignalBroadcastOnce("startup")
		for {
			d := durationUntilNextClock(hour, 0, loc)
			log.Printf("signals: next broadcast in %v (at %02d:00 %s)", d, hour, loc.String())
			time.Sleep(d)
			runSignalBroadcastOnce("scheduled")
		}
	}()
}

func runSignalBroadcastOnce(reason string) {
	log.Printf("signals: running broadcast (%s)", reason)
	if err := RunSignalBroadcast(); err != nil {
		log.Printf("signals: broadcast error: %v", err)
	}
}
