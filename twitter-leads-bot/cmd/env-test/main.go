// Command env-test verifies that .env loads and the Twitter scraper cookies
// are visible to the process. Run with:
//
//	go run ./cmd/env-test
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	printVar("TWITTER_AUTH_TOKEN")
	printVar("TWITTER_CSRF_TOKEN")
}

// printVar prints a "<name>=<value>" line, marking empty values so missing
// keys are obvious instead of showing as a blank line.
func printVar(name string) {
	v := os.Getenv(name)
	if v == "" {
		fmt.Printf("%s=(empty)\n", name)
		return
	}
	fmt.Printf("%s=%s\n", name, v)
}
