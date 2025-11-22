package ethereum

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"nft-auction-backend/internal/storage"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ListenToEvents subscribes to AuctionCreated, AuctionEnded, and BidPlaced events.
// 現在它接受一個 RedisClient 實例，用於快速存儲和更新實時競標數據。
func ListenToEvents(client *ethclient.Client, contract *Ethereum, db *sql.DB, rdb *storage.RedisClient) {
	ctx := context.Background()

	// --- 訂閱設置（與之前相同） ---
	// AuctionCreated
	createdCh := make(chan *EthereumAuctionCreated)
	createdSub, err := contract.WatchAuctionCreated(&bind.WatchOpts{Context: ctx}, createdCh, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to AuctionCreated: %v", err)
	}
	defer createdSub.Unsubscribe()

	// AuctionEnded
	endedCh := make(chan *EthereumAuctionEnded)
	endedSub, err := contract.WatchAuctionEnded(&bind.WatchOpts{Context: ctx}, endedCh, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to AuctionEnded: %v", err)
	}
	defer endedSub.Unsubscribe()

	// BidPlaced
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
			log.Printf("📢 AuctionCreated: AuctionID=%v, TokenID=%v, Seller=%s, StartPrice=%v, EndTime=%v",
				ev.AuctionId, ev.TokenId, ev.Seller.Hex(), ev.StartPrice, ev.EndTime)

			// 1. MySQL 寫入 (新增拍賣記錄)
			_, err := db.Exec(`
                INSERT INTO auctions (auction_id, token_id, seller, start_price, end_time, current_price)
                VALUES (?, ?, ?, ?, ?, ?)`,
				ev.AuctionId.Int64(),
				ev.TokenId.Int64(),
				ev.Seller.Hex(),
				ev.StartPrice.Int64(),
				ev.EndTime.Int64(),
				ev.StartPrice.Int64()) // 初始價格也是當前價格
			if err != nil {
				log.Printf("DB insert error (AuctionCreated): %v", err)
			}

			// 2. Redis 寫入 (設置初始最高出價)
			// Key: auction:{auctionId}:highest_bid
			redisKey := fmt.Sprintf("auction:%d:highest_bid", ev.AuctionId.Int64())
			// 存儲出價金額（Value）和出價者地址（Field）作為一個 Hash 結構
			// 這樣 Web API 就可以快速獲取當前最高價和領先者
			err = rdb.Client.HSet(rdb.Ctx, redisKey, map[string]interface{}{
				"amount": ev.StartPrice.Int64(),
				"bidder": ev.Seller.Hex(), // 初始最高價不一定需要設置 bidder，但我們可以用 StartPrice 作為初始值
			}).Err()
			if err != nil {
				log.Printf("Redis initial set error (AuctionCreated): %v", err)
			}

		// AuctionEnded event
		case ev := <-endedCh:
			log.Printf("🏁 AuctionEnded: AuctionID=%v, Winner=%s, FinalPrice=%v",
				ev.AuctionId, ev.Winner.Hex(), ev.FinalPrice)

			// 1. MySQL 寫入 (更新拍賣結果)
			_, err := db.Exec(`
                UPDATE auctions
                SET winner=?, final_price=?, ended=1
                WHERE auction_id=?`,
				ev.Winner.Hex(), ev.FinalPrice.Int64(), ev.AuctionId.Int64())
			if err != nil {
				log.Printf("DB update error (AuctionEnded): %v", err)
			}

			// 2. Redis 清理 (移除實時最高出價)
			redisKey := fmt.Sprintf("auction:%d:highest_bid", ev.AuctionId.Int64())
			err = rdb.Client.Del(rdb.Ctx, redisKey).Err()
			if err != nil {
				log.Printf("Redis delete error (AuctionEnded): %v", err)
			}

		// BidPlaced event
		case ev := <-bidCh:
			log.Printf("💰 BidPlaced: AuctionID=%v, Bidder=%s, Amount=%v",
				ev.AuctionId, ev.Bidder.Hex(), ev.Amount)

			// 1. MySQL 寫入 (新增出價記錄)
			_, err := db.Exec(`
                INSERT INTO bids (auction_id, bidder, amount)
                VALUES (?, ?, ?)`,
				ev.AuctionId.Int64(), ev.Bidder.Hex(), ev.Amount.Int64())
			if err != nil {
				log.Printf("DB insert error (BidPlaced): %v", err)
			}

			// 2. MySQL 寫入 (更新拍賣當前最高價)
			_, err = db.Exec(`
				UPDATE auctions
				SET current_price=?, current_bidder=?
				WHERE auction_id=?`,
				ev.Amount.Int64(), ev.Bidder.Hex(), ev.AuctionId.Int64())
			if err != nil {
				log.Printf("DB update error (BidPlaced - Current Price): %v", err)
			}

			// 3. Redis 寫入 (更新實時最高出價)
			redisKey := fmt.Sprintf("auction:%d:highest_bid", ev.AuctionId.Int64())
			err = rdb.Client.HSet(rdb.Ctx, redisKey, map[string]interface{}{
				"amount": ev.Amount.Int64(),
				"bidder": ev.Bidder.Hex(),
			}).Err()
			if err != nil {
				log.Printf("Redis update error (BidPlaced): %v", err)
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
