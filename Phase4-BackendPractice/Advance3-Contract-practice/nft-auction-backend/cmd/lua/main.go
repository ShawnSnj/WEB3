package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

// Define custom errors for better error handling in Go
var (
	ErrStockInsufficient = errors.New("stock insufficient (庫存不足)")
	ErrAlreadyPurchased  = errors.New("user already purchased (用戶重複購買)")
)

// The Redis Lua script for atomic purchasing logic.
// KEYS[1]: stock_key (String)
// KEYS[2]: buyer_set_key (Set)
// ARGV[1]: user_id (String)
const luaScript = `
local stock = tonumber(redis.call("GET", KEYS[1]))
if not stock or stock <= 0 then
    return -1 -- Stock is insufficient
end

local bought = redis.call("SISMEMBER", KEYS[2], ARGV[1])
if bought == 1 then
    return -2 -- User has already purchased
end

-- Atomic operation: Deduct stock and record purchase
redis.call("DECR", KEYS[1])
redis.call("SADD", KEYS[2], ARGV[1])

return 1 -- Success
`

// executePurchase calls the Lua script and handles the integer response codes.
func executePurchase(ctx context.Context, rdb *redis.Client, stockKey, buyerSetKey, userID string) error {
	// The KEYS and ARGV arrays for the EVAL command
	keys := []string{stockKey, buyerSetKey}
	args := []interface{}{userID}

	// Use Eval to execute the script. The result is returned as an interface{}.
	// The Lua script is guaranteed to return an integer (1, -1, or -2).
	result, err := rdb.Eval(ctx, luaScript, keys, args...).Result()

	// Check for communication/execution errors first
	if err != nil {
		log.Printf("Redis EVAL error: %v", err)
		return fmt.Errorf("redis execution failed: %w", err)
	}

	// Convert the result to an int64 for comparison
	purchaseCode, ok := result.(int64)
	if !ok {
		// This should not happen if the Lua script is correct
		return fmt.Errorf("unexpected return type from Lua script: %T", result)
	}

	// Handle the codes returned by the Lua script
	switch purchaseCode {
	case 1:
		log.Printf("User %s successfully purchased the item.", userID)
		return nil // Success
	case -1:
		return ErrStockInsufficient
	case -2:
		return ErrAlreadyPurchased
	default:
		return fmt.Errorf("unknown purchase code returned: %d", purchaseCode)
	}
}

func main() {
	ctx := context.Background()

	// 1. Initialize Redis Client
	// NOTE: Replace the address with your actual Redis server address
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // e.g., "redis:6379" in a Docker environment
		Password: "",               // no password set
		DB:       0,                // use default DB
	})

	// PING to check connectivity
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	fmt.Println("Successfully connected to Redis.")

	// 2. Define KEYS and ARGS for the script
	const itemID = "product:123"
	const stockKey = itemID + ":stock"
	const buyerSetKey = itemID + ":buyers"
	const initialStock = 2

	// 3. Reset state for demo
	// Set initial stock to 2
	rdb.Set(ctx, stockKey, initialStock, 0)
	// Clear the buyers set
	rdb.Del(ctx, buyerSetKey)
	fmt.Printf("\n--- Initial State: Stock set to %d, Buyers cleared ---\n", initialStock)

	// --- DEMO SCENARIOS ---

	// Scenario 1: User A purchases (Success: returns 1)
	fmt.Println("\n[Scenario 1] User A attempts to purchase (Stock: 2)")
	err = executePurchase(ctx, rdb, stockKey, buyerSetKey, "user_A")
	if err != nil {
		fmt.Printf("User A purchase failed: %v\n", err)
	} else {
		fmt.Println("User A: SUCCESS")
	}

	// Scenario 2: User A attempts to repurchase (Failure: returns -2)
	fmt.Println("\n[Scenario 2] User A attempts to repurchase (Stock: 1)")
	err = executePurchase(ctx, rdb, stockKey, buyerSetKey, "user_A")
	if errors.Is(err, ErrAlreadyPurchased) {
		fmt.Printf("User A: FAILURE (Expected) -> %v\n", err)
	} else if err != nil {
		fmt.Printf("User A: FAILED with unexpected error: %v\n", err)
	} else {
		fmt.Println("User A: SUCCESS (Unexpected)")
	}

	// Scenario 3: User B purchases (Success: returns 1, Stock exhausted)
	fmt.Println("\n[Scenario 3] User B attempts to purchase (Stock: 1)")
	err = executePurchase(ctx, rdb, stockKey, buyerSetKey, "user_B")
	if err != nil {
		fmt.Printf("User B purchase failed: %v\n", err)
	} else {
		fmt.Println("User B: SUCCESS")
	}

	// Scenario 4: User C attempts to purchase (Failure: returns -1)
	fmt.Println("\n[Scenario 4] User C attempts to purchase (Stock: 0)")
	err = executePurchase(ctx, rdb, stockKey, buyerSetKey, "user_C")
	if errors.Is(err, ErrStockInsufficient) {
		fmt.Printf("User C: FAILURE (Expected) -> %v\n", err)
	} else if err != nil {
		fmt.Printf("User C: FAILED with unexpected error: %v\n", err)
	} else {
		fmt.Println("User C: SUCCESS (Unexpected)")
	}

	// 4. Print final state
	finalStock, _ := rdb.Get(ctx, stockKey).Int64()
	finalBuyers, _ := rdb.SMembers(ctx, buyerSetKey).Result()
	fmt.Printf("\n--- Final State ---\n")
	fmt.Printf("Final Stock: %d\n", finalStock)
	fmt.Printf("Final Buyers: %v\n", finalBuyers)
}
