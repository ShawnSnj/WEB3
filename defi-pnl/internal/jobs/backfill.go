package jobs

import (
	"bytes"
	"defi-pnl/internal/storage"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

const uniswapV3SubgraphID = "5zvR82QoaXYFyDEKLZ9t6v9adgnptxYpKpSbxtgVENFV"

const (
	txTypeUser = "U" // aggregated by Swap.origin
	txTypeBot  = "B" // aggregated by Swap.sender
)

var warnedMissingEndpoint bool

type Swap struct {
	Origin    string `json:"origin"`
	Sender    string `json:"sender"`
	AmountUSD string `json:"amountUSD"`
	Timestamp string `json:"timestamp"`
}

type GraphQLResp struct {
	Data struct {
		Swaps []Swap `json:"swaps"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func RunBackfill(start time.Time, days int) {
	for i := 0; i < days; i++ {
		dayStart := start.AddDate(0, 0, i)
		RunBackfillDay(dayStart)
	}
}

// RunBackfillDay fetches and stores leaderboard for one calendar day (local midnight → +24h).
func RunBackfillDay(dayStart time.Time) {
	dayEnd := dayStart.Add(24 * time.Hour)

	exists, err := storage.DailyLeaderboardExists(dayStart)
	if err != nil {
		fmt.Printf("Check daily_leaderboard error for %s: %v\n", dayStart.Format("2006-01-02"), err)
		return
	}
	if exists {
		fmt.Println("Skipping existing day:", dayStart.Format("2006-01-02"))
		return
	}

	fmt.Println("Processing:", dayStart.Format("2006-01-02"))
	processOneDay(dayStart, dayEnd)
}

// StartDailyLeaderboardScheduler runs RunBackfillDay for yesterday every day at `hour` (0–23) in `loc`.
// At 02:00, yesterday is the last completed local calendar day.
func StartDailyLeaderboardScheduler(hour int, loc *time.Location) {
	if loc == nil {
		loc = time.Local
	}
	if hour < 0 || hour > 23 {
		hour = 2
	}
	go func() {
		for {
			d := durationUntilNextClock(hour, 0, loc)
			log.Printf("daily leaderboard: next run in %v (at %02d:00 %s)", d, hour, loc.String())
			time.Sleep(d)
			yesterday := calendarMidnight(time.Now().In(loc)).AddDate(0, 0, -1)
			log.Printf("daily leaderboard: running for %s", yesterday.Format("2006-01-02"))
			RunBackfillDay(yesterday)
		}
	}()
}

func calendarMidnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func durationUntilNextClock(hour, minute int, loc *time.Location) time.Duration {
	now := time.Now().In(loc)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}

func processOneDay(start, end time.Time) {
	senderVol := make(map[string]float64)
	originVol := make(map[string]float64)

	lastTimestamp := start.Unix()

	for {
		swaps := fetchPage(lastTimestamp, end.Unix())

		if len(swaps) == 0 {
			break
		}

		for _, s := range swaps {
			amt := parseFloat(s.AmountUSD)
			if s.Sender != "" {
				senderVol[s.Sender] += amt
			}
			if s.Origin != "" {
				originVol[s.Origin] += amt
			}

			ts := parseInt(s.Timestamp)
			if ts > lastTimestamp {
				lastTimestamp = ts
			}
		}

		lastTimestamp++
	}

	topByOrigin := topNLeaderboard(originVol, 10, txTypeUser)
	topBySender := topNLeaderboard(senderVol, 10, txTypeBot)
	entries := make([]storage.LeaderboardEntry, 0, len(topByOrigin)+len(topBySender))
	entries = append(entries, topByOrigin...)
	entries = append(entries, topBySender...)

	if len(entries) == 0 {
		fmt.Println("No leaderboard entries:", start.Format("2006-01-02"))
		return
	}
	if err := storage.InsertDailyLeaderboard(start, entries); err != nil {
		fmt.Println("InsertDailyLeaderboard error:", err)
		return
	}
	fmt.Printf("Inserted %d leaderboard rows for %s (10 U + 10 B)\n", len(entries), start.Format("2006-01-02"))
}

func fetchPage(from, to int64) []Swap {
	endpoint := graphEndpoint()
	if endpoint == "" {
		if !warnedMissingEndpoint {
			fmt.Println("Graph endpoint not configured. Set GRAPH_API_KEY or GRAPH_ENDPOINT.")
			warnedMissingEndpoint = true
		}
		return nil
	}

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
        origin
        sender
        amountUSD
        timestamp
      }
    }`, from, to)

	body, _ := json.Marshal(map[string]string{"query": query})

	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Graph request error:", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		fmt.Printf("Graph response status %d: %s\n", resp.StatusCode, string(raw))
		return nil
	}

	var result GraphQLResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Println("Graph decode error:", err)
		return nil
	}
	if len(result.Errors) > 0 {
		fmt.Println("GraphQL error:", result.Errors[0].Message)
		return nil
	}

	swaps := result.Data.Swaps
	logSubgraphFetch(from, to, swaps)
	return swaps
}

func graphEndpoint() string {
	if endpoint := os.Getenv("GRAPH_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	if apiKey := os.Getenv("GRAPH_API_KEY"); apiKey != "" {
		return fmt.Sprintf("https://gateway.thegraph.com/api/%s/subgraphs/id/%s", apiKey, uniswapV3SubgraphID)
	}
	return ""
}

func topNLeaderboard(m map[string]float64, n int, txType string) []storage.LeaderboardEntry {
	var list []storage.LeaderboardEntry
	for k, v := range m {
		list = append(list, storage.LeaderboardEntry{TxAddress: k, Volume: v, TxType: txType})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Volume > list[j].Volume
	})
	if len(list) > n {
		list = list[:n]
	}
	return list
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseInt(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
