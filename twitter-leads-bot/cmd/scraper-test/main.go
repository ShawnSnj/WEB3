// Command scraper-test is a smoke test for github.com/n0madic/twitter-scraper.
//
// Twitter/X no longer permits anonymous search, so the scraper must be
// authenticated. We support two ways, in order of preference:
//
//  1. Auth cookies (TWITTER_AUTH_TOKEN + TWITTER_CSRF_TOKEN). Grab these from
//     a logged-in browser session: cookie names are auth_token and ct0.
//  2. Username/password (TWITTER_USERNAME + TWITTER_PASSWORD, optionally
//     TWITTER_EMAIL for confirmation). Often blocked by X's anti-bot checks.
//
// Run with:
//
//	go run ./cmd/scraper-test                 # default query "polymarket", 20 tweets
//	go run ./cmd/scraper-test "ai agents" 10  # custom query and count
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"time"
	"unsafe"

	"github.com/joho/godotenv"
	twitterscraper "github.com/imperatrona/twitter-scraper"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf(".env load: %v (continuing with process env)", err)
	}

	query := "polymarket"
	count := 20
	if len(os.Args) > 1 {
		query = os.Args[1]
	}
	if len(os.Args) > 2 {
		if n, err := strconv.Atoi(os.Args[2]); err == nil && n > 0 {
			count = n
		}
	}

	scraper := twitterscraper.New()
	if err := authenticate(scraper); err != nil {
		log.Fatalf("auth: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Printf("searching %q (max %d)…", query, count)

	got, errs := 0, 0
	for result := range scraper.SearchTweets(ctx, query, count) {
		if result.Error != nil {
			errs++
			log.Printf("scrape error: %v", result.Error)
			continue
		}
		got++
		fmt.Println("===================================")
		fmt.Println("User:    ", result.Username)
		fmt.Println("Text:    ", result.Text)
		fmt.Println("Likes:   ", result.Likes)
		fmt.Println("URL:     ", result.PermanentURL)
	}

	log.Printf("done: %d tweet(s) returned, %d error(s)", got, errs)
}

// authenticate uses cookie auth (auth_token + ct0) via SetAuthToken, which
// also flips the scraper's internal isLogged flag — required because the
// search endpoint refuses to fire otherwise. Username/password login is the
// fallback, but is frequently blocked by X's anti-bot checks.
func authenticate(s *twitterscraper.Scraper) error {
	if authToken, csrf := os.Getenv("TWITTER_AUTH_TOKEN"), os.Getenv("TWITTER_CSRF_TOKEN"); authToken != "" && csrf != "" {
		log.Printf("auth: cookies loaded — auth_token len=%d, ct0 len=%d", len(authToken), len(csrf))
		s.SetAuthToken(twitterscraper.AuthToken{Token: authToken, CSRFToken: csrf})

		// IsLoggedIn() pings api.twitter.com/1.1/account/verify_credentials.json
		// which X is in the process of deprecating — it returns 401 even for
		// otherwise-valid cookies. We bypass it by flipping the unexported
		// `isLogged` flag directly, then let the actual search request prove
		// whether the cookies work.
		forceLoggedIn(s)
		log.Println("auth: forced isLogged=true; relying on search to validate cookies")
		return nil
	}

	if user, pass := os.Getenv("TWITTER_USERNAME"), os.Getenv("TWITTER_PASSWORD"); user != "" && pass != "" {
		creds := []string{user, pass}
		if email := os.Getenv("TWITTER_EMAIL"); email != "" {
			creds = append(creds, email)
		}
		if err := s.Login(creds...); err != nil {
			return fmt.Errorf("login as %s: %w", user, err)
		}
		log.Printf("auth: logged in as %s", user)
		return nil
	}

	return errors.New("no credentials: set TWITTER_AUTH_TOKEN+TWITTER_CSRF_TOKEN or TWITTER_USERNAME+TWITTER_PASSWORD")
}

// forceLoggedIn sets the scraper's unexported `isLogged` field to true via
// reflection. This is a deliberate workaround for the fork's IsLoggedIn()
// precheck, which is unreliable now that X is deprecating the v1.1 auth
// endpoints. If the cookies are bad, the actual search call will surface the
// error per-result via result.Error.
func forceLoggedIn(s *twitterscraper.Scraper) {
	v := reflect.ValueOf(s).Elem()
	f := v.FieldByName("isLogged")
	if !f.IsValid() {
		log.Println("warn: isLogged field not found — fork layout may have changed")
		return
	}
	reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem().SetBool(true)
}

