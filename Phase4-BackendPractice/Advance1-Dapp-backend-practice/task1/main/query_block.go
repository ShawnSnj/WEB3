package main

import (
	"context"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
)

func QueryBlock() {
	client, err := ethclient.Dial("https://sepolia.infura.io/v3/<API_KEY>")
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	defer client.Close()

	blockNumber := big.NewInt(1234567)

	block, err := client.BlockByNumber(context.Background(), blockNumber)
	if err != nil {
		log.Fatalf("Failed to retrieve block: %v", err)
	}

	fmt.Printf("Block Number: %d\n", block.Number().Uint64())
	fmt.Printf("Block Hash: %s\n", block.Hash().Hex())
	fmt.Printf("Block Time: %d\n", block.Time())
	fmt.Printf("Number of Transactions	: %d\n", len(block.Transactions()))
}
