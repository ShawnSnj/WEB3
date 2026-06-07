// Command server boots the Twitter Leads Bot.
//
// All third-party clients (Postgres, Twitter searcher, Gemini) are
// constructed here and injected into the pipeline + web layer. Keeping wiring
// at the edge means inner packages stay free of global state.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/shawn/twitter-leads-bot/internal/db"
	"github.com/shawn/twitter-leads-bot/internal/gemini"
	"github.com/shawn/twitter-leads-bot/internal/pipeline"
	"github.com/shawn/twitter-leads-bot/internal/twitter"
	"github.com/shawn/twitter-leads-bot/internal/web"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, ".env load: %v (continuing with process env)\n", err)
	}

	cfg := loadConfig()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	repo, err := db.New(cfg.DatabaseURL)
	if err != nil {
		logger.Error("db connect", slog.Any("err", err))
		os.Exit(1)
	}
	defer repo.Close()

	searcher := newSearcher(cfg, logger)
	if cfg.GeminiKey == "" {
		logger.Error("GEMINI_API_KEY is required")
		os.Exit(1)
	}
	analyzer, err := gemini.New(cfg.GeminiKey, cfg.GeminiModel)
	if err != nil {
		logger.Error("gemini client", slog.Any("err", err))
		os.Exit(1)
	}
	poster := pipeline.NewPoster(os.Getenv("TWITTER_REPLY_WEBHOOK_URL"), os.Getenv("TWITTER_REPLY_WEBHOOK_SECRET"), logger)
	replyViaWebhook := strings.TrimSpace(os.Getenv("TWITTER_REPLY_WEBHOOK_URL")) != ""

	displayLoc, err := time.LoadLocation(env("DISPLAY_TIMEZONE", "Asia/Taipei"))
	if err != nil {
		logger.Warn("invalid DISPLAY_TIMEZONE, using Asia/Taipei", slog.String("tz", os.Getenv("DISPLAY_TIMEZONE")), slog.Any("err", err))
		displayLoc, _ = time.LoadLocation("Asia/Taipei")
	}

	pipe := pipeline.New(repo, searcher, analyzer,
		logger.With(slog.String("component", "pipeline")),
		pipeline.Config{
			ManualOnly:       !cfg.PipelineAutoRun,
			Interval:         cfg.PipelineInterval,
			Workers:          cfg.PipelineWorkers,
			KeywordStagger:   cfg.KeywordStagger,
			PerKeywordMax:    cfg.PerKeywordMax,
			MinLikes:         3,
			MinTextLen:       40,
			MaxRequestJitter: 2 * time.Second,
		})

	srv, err := web.NewServer(repo, pipe, poster, logger.With(slog.String("component", "web")), replyViaWebhook, displayLoc)
	if err != nil {
		logger.Error("web server init", slog.Any("err", err))
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Pipeline runs in its own goroutine, gets cancelled on shutdown.
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()
	go pipe.Run(rootCtx)

	go func() {
		logger.Info("listening", slog.String("addr", "http://localhost:"+cfg.Port))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http listen", slog.Any("err", err))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down")

	// Stop accepting new requests; in-flight requests get up to 10s to finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	rootCancel() // tell the pipeline loop to exit
}

// ---- config -----------------------------------------------------------------

type config struct {
	Port             string
	DatabaseURL      string
	LogLevel         slog.Level
	TwitterBearer    string
	RapidAPIKey      string
	RapidAPIHost     string
	RapidAPIKeys     []string
	RapidAPIPool     []string
	RapidAPICooldown time.Duration
	GeminiKey        string
	GeminiModel      string
	PipelineAutoRun  bool // background ticker; default off — use "Run search" on /keywords
	PipelineInterval time.Duration
	PipelineWorkers  int
	KeywordStagger   time.Duration
	PerKeywordMax    int
}

func loadConfig() config {
	c := config{
		Port:             env("PORT", "8080"),
		DatabaseURL:      env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/twitter_leads?sslmode=disable"),
		LogLevel:         parseLevel(os.Getenv("LOG_LEVEL")),
		TwitterBearer:    os.Getenv("TWITTER_BEARER_TOKEN"),
		RapidAPIKey:      os.Getenv("RAPIDAPI_KEY"),
		RapidAPIHost:     os.Getenv("RAPIDAPI_HOST"),
		RapidAPIKeys:     splitCSV(os.Getenv("RAPIDAPI_KEYS")),
		RapidAPIPool:     splitCSV(os.Getenv("RAPIDAPI_POOL")),
		RapidAPICooldown: parseDuration(os.Getenv("RAPIDAPI_COOLDOWN"), time.Hour),
		GeminiKey:        os.Getenv("GEMINI_API_KEY"),
		GeminiModel:      env("GEMINI_MODEL", "gemini-2.5-flash"),
		PipelineAutoRun:  parseBool(os.Getenv("PIPELINE_AUTO_RUN")),
		PipelineInterval: parseDuration(os.Getenv("PIPELINE_INTERVAL"), 5*time.Minute),
		PipelineWorkers:  parseInt(os.Getenv("PIPELINE_WORKERS"), 1),
		KeywordStagger:   parseDuration(os.Getenv("PIPELINE_KEYWORD_STAGGER"), 5*time.Second),
		PerKeywordMax:    parseInt(os.Getenv("PIPELINE_PER_KEYWORD_MAX"), 20),
	}
	return c
}

// newSearcher picks the best available tweet source, in priority order:
//
//  1. RAPIDAPI_POOL — mixed-provider pool of "host|key" entries.
//  2. RAPIDAPI_KEYS — multiple keys for the same provider (free-tier rotation).
//  3. RAPIDAPI_KEY  — single RapidAPI client.
//  4. TWITTER_BEARER_TOKEN — official X v2 API.
func newSearcher(cfg config, logger *slog.Logger) twitter.Searcher {
	if len(cfg.RapidAPIPool) > 0 {
		members := make([]twitter.PoolMember, 0, len(cfg.RapidAPIPool))
		for i, entry := range cfg.RapidAPIPool {
			host, key, ok := strings.Cut(entry, "|")
			if !ok || host == "" || key == "" {
				logger.Warn("skipping malformed RAPIDAPI_POOL entry", slog.Int("index", i))
				continue
			}
			members = append(members, twitter.PoolMember{
				Name:     fmt.Sprintf("%s#%d", host, i),
				Searcher: twitter.NewRapidAPI(key, host),
			})
		}
		logger.Info("twitter source", slog.String("kind", "rapidapi-pool"), slog.Int("members", len(members)))
		return twitter.NewPool(members, cfg.RapidAPICooldown)
	}

	if len(cfg.RapidAPIKeys) > 0 {
		members := make([]twitter.PoolMember, 0, len(cfg.RapidAPIKeys))
		for i, key := range cfg.RapidAPIKeys {
			members = append(members, twitter.PoolMember{
				Name:     fmt.Sprintf("rapidapi-key#%d", i),
				Searcher: twitter.NewRapidAPI(key, cfg.RapidAPIHost),
			})
		}
		logger.Info("twitter source", slog.String("kind", "rapidapi-keys"),
			slog.Int("keys", len(members)), slog.Duration("cooldown", cfg.RapidAPICooldown))
		return twitter.NewPool(members, cfg.RapidAPICooldown)
	}

	if cfg.RapidAPIKey != "" {
		logger.Info("twitter source", slog.String("kind", "rapidapi-single"))
		return twitter.NewRapidAPI(cfg.RapidAPIKey, cfg.RapidAPIHost)
	}

	logger.Info("twitter source", slog.String("kind", "twitter-v2"))
	return twitter.New(cfg.TwitterBearer)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return fallback
}

func parseInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
		return n
	}
	return fallback
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
