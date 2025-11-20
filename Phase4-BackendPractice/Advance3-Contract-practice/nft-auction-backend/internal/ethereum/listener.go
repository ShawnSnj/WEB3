package ethereum

import (
	"context"
	"database/sql"
	"log"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ListenToEvents subscribes to AuctionCreated, AuctionEnded, and BidPlaced events.
func ListenToEvents(client *ethclient.Client, contract *Ethereum, db *sql.DB) {
	ctx := context.Background()

	// --- AuctionCreated ---
	createdCh := make(chan *EthereumAuctionCreated)
	// NOTE: Assuming the WatchAuctionCreated method includes AuctionId in the nil filters below
	createdSub, err := contract.WatchAuctionCreated(&bind.WatchOpts{Context: ctx}, createdCh, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to AuctionCreated: %v", err)
	}
	defer createdSub.Unsubscribe()

	// --- AuctionEnded ---
	endedCh := make(chan *EthereumAuctionEnded)
	endedSub, err := contract.WatchAuctionEnded(&bind.WatchOpts{Context: ctx}, endedCh, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to AuctionEnded: %v", err)
	}
	defer endedSub.Unsubscribe()

	// --- BidPlaced ---
	bidCh := make(chan *EthereumBidPlaced)
	bidSub, err := contract.WatchBidPlaced(&bind.WatchOpts{Context: ctx}, bidCh, nil, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to BidPlaced: %v", err)
	}
	defer bidSub.Unsubscribe()

	log.Println("🔔 Listening for events...")

	for {
		select {
		// AuctionCreated event
		case ev := <-createdCh:
			// 1. Log Correction: Include the AuctionId in the output
			log.Printf("📢 AuctionCreated: AuctionID=%v, TokenID=%v, Seller=%s, StartPrice=%v, EndTime=%v",
				ev.AuctionId, ev.TokenId, ev.Seller.Hex(), ev.StartPrice, ev.EndTime)

			// 2. INSERT Correction: Add 'auction_id' column and 'ev.AuctionId' value
			_, err := db.Exec(`
                INSERT INTO auctions (auction_id, token_id, seller, start_price, end_time)
                VALUES (?, ?, ?, ?, ?)`,
				ev.AuctionId.Int64(), // <-- FIX: Include AuctionId for the primary key
				ev.TokenId.Int64(),
				ev.Seller.Hex(),
				ev.StartPrice.Int64(),
				ev.EndTime.Int64())
			if err != nil {
				log.Printf("DB insert error (AuctionCreated): %v", err)
			}

		// AuctionEnded event
		case ev := <-endedCh:
			log.Printf("🏁 AuctionEnded: AuctionID=%v, Winner=%s, FinalPrice=%v",
				ev.AuctionId, ev.Winner.Hex(), ev.FinalPrice)

			_, err := db.Exec(`
                UPDATE auctions
                SET winner=?, final_price=?, ended=1
                WHERE auction_id=?`,
				ev.Winner.Hex(), ev.FinalPrice.Int64(), ev.AuctionId.Int64())
			if err != nil {
				log.Printf("DB update error (AuctionEnded): %v", err)
			}

		// BidPlaced event
		case ev := <-bidCh:
			log.Printf("💰 BidPlaced: AuctionID=%v, Bidder=%s, Amount=%v",
				ev.AuctionId, ev.Bidder.Hex(), ev.Amount)

			_, err := db.Exec(`
                INSERT INTO bids (auction_id, bidder, amount)
                VALUES (?, ?, ?)`,
				ev.AuctionId.Int64(), ev.Bidder.Hex(), ev.Amount.Int64())
			if err != nil {
				log.Printf("DB insert error (BidPlaced): %v", err)
			}

		// Subscription errors
		case err := <-createdSub.Err():
			log.Printf("⚠️ AuctionCreated subscription error: %v", err)
		case err := <-endedSub.Err():
			log.Printf("⚠️ AuctionEnded subscription error: %v", err)
		case err := <-bidSub.Err():
			log.Printf("⚠️ BidPlaced subscription error: %v", err)

		case <-ctx.Done():
			log.Println("Listener stopped.")
			return
		}
	}
}
