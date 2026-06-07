// Command rapidapi-test is a smoke test for the RapidAPI Twitter searcher.
//
// Subscribe to a Twitter scraper on https://rapidapi.com (e.g. "Twitter API
// 4.5"), grab the API key, and put it in .env:
//
//	RAPIDAPI_KEY=your-key-here
//	# RAPIDAPI_HOST=twitter-api45.p.rapidapi.com   (optional, defaults to this)
//
// Run with:
//
//	go run ./cmd/rapidapi-test                 # default query "polymarket", 5 tweets
//	go run ./cmd/rapidapi-test "ai agents" 10  # custom query and count
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"github.com/shawn/twitter-leads-bot/internal/twitter"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf(".env load: %v (continuing with process env)", err)
	}

	apiKey := os.Getenv("RAPIDAPI_KEY")
	if apiKey == "" {
		log.Fatal("RAPIDAPI_KEY is required (subscribe to a Twitter scraper on rapidapi.com)")
	}

	query := "polymarket"
	count := 5
	if len(os.Args) > 1 {
		query = os.Args[1]
	}
	if len(os.Args) > 2 {
		if n, err := strconv.Atoi(os.Args[2]); err == nil && n > 0 {
			count = n
		}
	}

	client := twitter.NewRapidAPI(apiKey, os.Getenv("RAPIDAPI_HOST"))

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	log.Printf("searching %q (max %d) via RapidAPI…", query, count)

	tweets, err := client.Search(ctx, query, count)
	if err != nil {
		log.Fatalf("search: %v", err)
	}

	for _, t := range tweets {
		fmt.Println("===================================")
		fmt.Println("User: ", t.Author)
		fmt.Println("Text: ", t.Text)
		fmt.Println("URL:  ", t.URL)
	}
	log.Printf("done: %d tweet(s) returned", len(tweets))
}
