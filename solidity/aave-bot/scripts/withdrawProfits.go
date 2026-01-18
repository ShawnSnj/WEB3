package main

import (
	"context"
	"crypto/ecdsa"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"aave-bot/integration"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

/**
 * Manual Profit Withdrawal Script
 * 
 * Withdraws profits from the flash loan contract to your wallet
 * 
 * Usage:
 *   go run scripts/withdrawProfits.go
 *   TOKEN_ADDRESS=0x... go run scripts/withdrawProfits.go  # Withdraw specific token
 */

func main() {
	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Get configuration
	contractAddress := os.Getenv("FLASHLOAN_CONTRACT_ADDRESS")
	if contractAddress == "" {
		log.Fatal("FLASHLOAN_CONTRACT_ADDRESS not set in .env")
	}

	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		log.Fatal("RPC_URL not set in .env")
	}

	privateKeyHex := os.Getenv("PRIVATE_KEY")
	if privateKeyHex == "" {
		log.Fatal("PRIVATE_KEY not set in .env")
	}

	// Connect to network
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Setup wallet
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("Failed to get public key")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	ctx := context.Background()

	// Get nonce and gas price
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		log.Fatalf("Failed to get nonce: %v", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatalf("Failed to get gas price: %v", err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("Failed to get chain ID: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		log.Fatalf("Failed to create transactor: %v", err)
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.GasPrice = gasPrice
	auth.GasLimit = 300000

	// Load flash loan contract
	flashLoan, err := integration.NewFlashLoanLiquidation(client, contractAddress)
	if err != nil {
		log.Fatalf("Failed to load contract: %v", err)
	}

	// Verify ownership
	owner, err := flashLoan.GetOwner(ctx)
	if err != nil {
		log.Fatalf("Failed to get owner: %v", err)
	}

	if owner != fromAddress {
		log.Fatalf("You are not the contract owner! Owner: %s, You: %s", owner.Hex(), fromAddress.Hex())
	}

	log.Printf("Contract: %s", contractAddress)
	log.Printf("Owner: %s (You)\n", owner.Hex())

	// Get token to withdraw
	tokenAddressStr := os.Getenv("TOKEN_ADDRESS")
	if tokenAddressStr == "" {
		log.Println("TOKEN_ADDRESS not set. Checking contract balance...")
		log.Println("Usage: TOKEN_ADDRESS=0x... go run scripts/withdrawProfits.go")
		log.Println("\nTo find aToken address, check Aave Pool reserve data")
		return
	}

	tokenAddress := common.HexToAddress(tokenAddressStr)

	// Check balance
	balance, err := flashLoan.GetContractBalance(ctx, tokenAddress)
	if err != nil {
		log.Fatalf("Failed to check balance: %v", err)
	}

	log.Printf("Token: %s", tokenAddress.Hex())
	log.Printf("Balance: %s\n", balance.String())

	if balance.Cmp(big.NewInt(0)) == 0 {
		log.Println("No balance to withdraw")
		return
	}

	// Withdraw
	log.Println("Withdrawing...")

	// Try aToken withdrawal first (redeems to underlying)
	txHash, err := flashLoan.WithdrawAToken(ctx, auth, tokenAddress)
	if err != nil {
		log.Printf("aToken withdrawal failed, trying regular withdraw: %v", err)
		txHash, err = flashLoan.Withdraw(ctx, auth, tokenAddress)
		if err != nil {
			log.Fatalf("Withdrawal failed: %v", err)
		}
	}

	log.Printf("Transaction sent: %s", txHash.Hex())
	log.Println("Waiting for confirmation...")

	// Wait for receipt
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			log.Fatal("Timeout waiting for transaction")
		case <-ticker.C:
			receipt, err := client.TransactionReceipt(ctx, *txHash)
			if err == nil {
				if receipt.Status == 0 {
					log.Fatal("Transaction reverted")
				}
				log.Printf("✓ Withdrawal confirmed in block %d", receipt.BlockNumber.Uint64())
				log.Printf("Gas used: %d", receipt.GasUsed)
				log.Println("\n✓ Profits withdrawn to your wallet!")
				return
			}
		}
	}
}
