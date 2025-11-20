package main

import (
	"log"
	"nft-auction-backend/internal/api"
	"nft-auction-backend/internal/config"
	"nft-auction-backend/internal/ethereum"
	"nft-auction-backend/internal/storage"

	"github.com/ethereum/go-ethereum/common"
)

func main() {
	cfg := config.Load()

	// MySQL
	db := storage.InitMySQL(cfg.MySQLDSN)
	defer db.Close()

	// Ethereum
	client := ethereum.NewClient(cfg.RPCURL)
	contractAddr := common.HexToAddress(cfg.ContractAddress)
	contract, err := ethereum.NewEthereum(contractAddr, client)
	if err != nil {
		log.Fatalf("Failed to bind contract: %v", err)
	}

	// Start event listener
	go ethereum.ListenToEvents(client, contract, db)

	// Start REST API server
	api.StartServer(cfg, db)
}
