package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Subgraph fetch logging (JSON Lines). Enable with:
//   SUBGRAPH_FETCH_LOG=1              → logs/subgraph_fetch.jsonl
//   SUBGRAPH_FETCH_LOG=/path/to/file  → append to that file
// Disable: SUBGRAPH_FETCH_LOG=0 or unset (default off to avoid huge files during long backfills).

var (
	subgraphLogMu   sync.Mutex
	subgraphLogFile *os.File
	subgraphLogErr  error // set on first failed open; nil means not tried or success
)

// InitSubgraphLog creates the log directory and an empty file when logging is enabled,
// so `tail -f logs/subgraph_fetch.jsonl` works before the first subgraph response.
func InitSubgraphLog() {
	path := subgraphFetchLogPath()
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Println("subgraph log init:", err)
			return
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("subgraph log init:", err)
		return
	}
	_ = f.Close()
}

func subgraphFetchLogPath() string {
	v := strings.TrimSpace(os.Getenv("SUBGRAPH_FETCH_LOG"))
	if v == "" || v == "0" || strings.EqualFold(v, "false") || strings.EqualFold(v, "off") {
		return ""
	}
	if v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") {
		return "logs/subgraph_fetch.jsonl"
	}
	return v
}

func openSubgraphLogLocked() (*os.File, error) {
	if subgraphLogFile != nil {
		return subgraphLogFile, nil
	}
	if subgraphLogErr != nil {
		return nil, subgraphLogErr
	}
	path := subgraphFetchLogPath()
	if path == "" {
		return nil, nil
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			subgraphLogErr = err
			fmt.Println("subgraph fetch log mkdir:", err)
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		subgraphLogErr = err
		fmt.Println("subgraph fetch log open:", err)
		return nil, err
	}
	subgraphLogFile = f
	return f, nil
}

type subgraphFetchRecord struct {
	FetchedAt   string `json:"fetched_at"`
	TimestampGt int64  `json:"timestamp_gt"`
	TimestampLt int64  `json:"timestamp_lt"`
	Count       int    `json:"count"`
	Swaps       []Swap `json:"swaps"`
}

// logSubgraphFetch appends one JSON object (one line) per successful Graph response page.
func logSubgraphFetch(from, to int64, swaps []Swap) {
	if subgraphFetchLogPath() == "" {
		return
	}
	rec := subgraphFetchRecord{
		FetchedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		TimestampGt: from,
		TimestampLt: to,
		Count:       len(swaps),
		Swaps:       swaps,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	b = append(b, '\n')

	subgraphLogMu.Lock()
	defer subgraphLogMu.Unlock()
	f, err := openSubgraphLogLocked()
	if err != nil || f == nil {
		return
	}
	_, _ = f.Write(b)
}
