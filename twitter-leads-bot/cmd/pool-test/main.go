// Command pool-test exercises a multi-key RapidAPI pool by issuing N searches
// in a row, so you can watch the pool round-robin across keys and park any
// member that hits its quota.
//
// Configure either of these in .env:
//
//	# Option A: multiple keys, same provider
//	RAPIDAPI_HOST=twitter-api45.p.rapidapi.com
//	RAPIDAPI_KEYS=key_one,key_two,key_three
//
//	# Option B: mix providers — each entry is "host|key"
//	RAPIDAPI_POOL=twitter-api45.p.rapidapi.com|key_one,twitter154.p.rapidapi.com|key_two
//
// Optional:
//
//	RAPIDAPI_COOLDOWN=1h   # how long a quota-exhausted member is parked
//
// Run:
//
//	go run ./cmd/pool-test                 # 3 searches of "polymarket"
//	go run ./cmd/pool-test "ai agents" 5   # 5 searches of "ai agents"
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/shawn/twitter-leads-bot/internal/twitter"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf(".env load: %v (continuing with process env)", err)
	}

	pool := buildPool()
	if pool == nil {
		log.Fatal("no pool members configured (set RAPIDAPI_POOL or RAPIDAPI_KEYS)")
	}
	log.Printf("pool ready with %d member(s)", pool.Len())

	query := "polymarket"
	rounds := 3
	if len(os.Args) > 1 {
		query = os.Args[1]
	}
	if len(os.Args) > 2 {
		if n, err := strconv.Atoi(os.Args[2]); err == nil && n > 0 {
			rounds = n
		}
	}

	for i := 1; i <= rounds; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		log.Printf("--- round %d/%d ---", i, rounds)
		tweets, err := pool.Search(ctx, query, 5)
		cancel()
		if err != nil {
			log.Printf("round %d: %v", i, err)
			continue
		}
		for _, t := range tweets {
			fmt.Printf("  @%s: %s\n", t.Author, oneLine(t.Text))
		}
	}
}

func buildPool() *twitter.Pool {
	cooldown := time.Hour
	if v := os.Getenv("RAPIDAPI_COOLDOWN"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cooldown = d
		}
	}

	if entries := splitCSV(os.Getenv("RAPIDAPI_POOL")); len(entries) > 0 {
		members := make([]twitter.PoolMember, 0, len(entries))
		for i, entry := range entries {
			host, key, ok := strings.Cut(entry, "|")
			if !ok {
				log.Printf("skip malformed RAPIDAPI_POOL entry %d", i)
				continue
			}
			members = append(members, twitter.PoolMember{
				Name:     fmt.Sprintf("%s#%d", host, i),
				Searcher: twitter.NewRapidAPI(key, host),
			})
		}
		if len(members) > 0 {
			return twitter.NewPool(members, cooldown)
		}
	}

	keys := splitCSV(os.Getenv("RAPIDAPI_KEYS"))
	if len(keys) == 0 {
		return nil
	}
	host := os.Getenv("RAPIDAPI_HOST")
	members := make([]twitter.PoolMember, 0, len(keys))
	for i, k := range keys {
		members = append(members, twitter.PoolMember{
			Name:     fmt.Sprintf("key#%d", i),
			Searcher: twitter.NewRapidAPI(k, host),
		})
	}
	return twitter.NewPool(members, cooldown)
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

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 140 {
		return s[:140] + "…"
	}
	return s
}
