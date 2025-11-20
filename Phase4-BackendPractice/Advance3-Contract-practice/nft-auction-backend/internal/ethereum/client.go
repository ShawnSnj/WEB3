package ethereum

import (
	"log"

	"github.com/ethereum/go-ethereum/ethclient"
)

func NewClient(rpcURL string) *ethclient.Client {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect Ethereum node: %v", err)
	}
	log.Println("✅ Connected to Ethereum node")
	return client
}
