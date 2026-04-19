package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"defi-pnl/internal/api"
	"defi-pnl/internal/jobs"
	"defi-pnl/internal/storage"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env from current working directory (run the server from repo root).
	if err := godotenv.Load(); err != nil {
		log.Printf("env: %v (using process environment only)", err)
	}
	jobs.InitSubgraphLog()

	err := storage.Init()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/user/pnl", api.GetPnL)
	http.HandleFunc("/leaderboard/daily/users", api.GetLeaderboardDailyUsers)
	http.HandleFunc("/leaderboard/daily/bots", api.GetLeaderboardDailyBots)
	http.HandleFunc("/leaderboard/daily", api.GetLeaderboardDaily)
	http.HandleFunc("/leaderboard/users", api.GetLeaderboardUsers)
	http.HandleFunc("/leaderboard/bots", api.GetLeaderboardBots)
	http.HandleFunc("/leaderboard", api.GetLeaderboard)

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v — port in use? stop the other process, or run PORT=8081 go run ./cmd/server", addr, err)
	}
	log.Printf("listening on %s", ln.Addr())

	// Backfill runs in the background so the API binds immediately and does not race with port 8080.
	start := time.Now().AddDate(-1, 0, 0)
	go jobs.RunBackfill(start, 365)

	log.Fatal(http.Serve(ln, nil))
}
