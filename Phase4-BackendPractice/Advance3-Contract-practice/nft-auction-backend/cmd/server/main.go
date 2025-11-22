package main

import (
	"fmt"
	"log"
	"nft-auction-backend/internal/api"
	"nft-auction-backend/internal/config"
	"nft-auction-backend/internal/ethereum"
	"nft-auction-backend/internal/storage"

	"github.com/ethereum/go-ethereum/common"
)

func main() {
	cfg := config.Load()

	// --- 1. MySQL (持久化數據庫) ---
	db := storage.InitMySQL(cfg.MySQLDSN)
	defer db.Close()

	// --- 2. Redis (緩存與實時數據庫) ---
	redisAddr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort) // 假設配置中包含 Host 和 Port
	rdb := storage.InitRedis(redisAddr, cfg.RedisPassword)          // 假設配置中包含 Password
	defer storage.CloseRedis(rdb)

	// --- 3. Ethereum (智能合約連接) ---
	client := ethereum.NewClient(cfg.RPCURL)
	contractAddr := common.HexToAddress(cfg.ContractAddress)
	contract, err := ethereum.NewEthereum(contractAddr, client)
	if err != nil {
		log.Fatalf("Failed to bind contract: %v", err)
	}

	// --- 4. 啟動事件監聽器 (需要 MySQL 和 Redis 進行讀寫) ---
	// 假設 ListenToEvents 函數需要 Redis 實例來儲存實時最高競標價
	go ethereum.ListenToEvents(client, contract, db, rdb)

	// --- 5. 啟動 REST API 服務器 (需要 MySQL 和 Redis 進行數據查詢) ---
	// 假設 StartServer 函數需要 Redis 實例來提供緩存和實時競標數據
	api.StartServer(cfg, db, rdb)
}
