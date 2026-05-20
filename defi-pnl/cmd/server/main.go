package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"defi-pnl/internal/api"
	"defi-pnl/internal/jobs"
	"defi-pnl/internal/storage"
	"defi-pnl/internal/telegram"

	"github.com/joho/godotenv"
)

const (
	envModeLocal  = "local"
	envModeRender = "render"
)

func main() {
	envMode := flag.String("env", defaultEnvMode(), `config source: "local" loads .env from the working directory; "render" uses process environment variables only (for Render.com / production)`)
	flag.Parse()

	loadConfig(*envMode)

	jobs.InitSubgraphLog()

	err := storage.Init()
	if err != nil {
		log.Fatal(err)
	}

	// Recompute today's pnl_leaderboard snapshot on every server start so the
	// API always serves a fresh value derived from whatever is currently in
	// trades_v2. The DELETE-then-INSERT in RunPnLLeaderboardTop10 makes this
	// safe to call repeatedly within the same calendar day.
	if err := jobs.RunPnLLeaderboardTop10(); err != nil {
		log.Printf("pnl: startup recalc error: %v", err)
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

	hour := 2
	if v := os.Getenv("DAILY_JOB_HOUR"); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h >= 0 && h <= 23 {
			hour = h
		}
	}

	signalHour := 3
	if v := os.Getenv("SIGNAL_HOUR"); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h >= 0 && h <= 23 {
			signalHour = h
		}
	}

	//jobs.StartDailyLeaderboardScheduler(hour, time.Local)
	jobs.StartDailyTradesScheduler(hour, time.Local)
	jobs.StartDailySignalScheduler(signalHour, time.Local)
	jobs.StartAlertScheduler()

	go telegram.StartTelegramBot()

	log.Fatal(http.Serve(ln, nil))
}

// loadConfig wires up environment variables according to the chosen mode.
// In "local" mode the .env file in the working directory is loaded so that
// developers can keep secrets out of their shell. In "render" mode the file
// is skipped and the process environment (populated by Render.com from its
// Environment Variables dashboard) is the single source of truth.
func loadConfig(mode string) {
	switch mode {
	case envModeLocal:
		if err := godotenv.Load(); err != nil {
			log.Printf("env[local]: .env not loaded (%v); using process env only", err)
			return
		}
		log.Printf("env[local]: loaded .env")
	case envModeRender:
		log.Printf("env[render]: using process environment variables only")
	default:
		log.Fatalf("env: unknown -env=%q (expected %q or %q)", mode, envModeLocal, envModeRender)
	}
}

// defaultEnvMode picks a sensible default when -env is not passed.
// APP_ENV wins if set; otherwise we auto-detect Render via the RENDER variable
// it injects into every service container, and fall back to "local".
func defaultEnvMode() string {
	if v := os.Getenv("APP_ENV"); v != "" {
		return v
	}
	if os.Getenv("RENDER") != "" {
		return envModeRender
	}
	return envModeLocal
}
