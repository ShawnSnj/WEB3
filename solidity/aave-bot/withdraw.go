package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"aave-bot/integration"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// withdrawProfits checks contract balance and withdraws if above threshold
func withdrawProfits(
	ctx context.Context,
	client *ethclient.Client,
	config *MonitorConfig,
	auth *bind.TransactOpts,
	botAddress common.Address,
) error {
	if !config.UseFlashLoanLiquidation {
		return nil // Only works with flash loan contracts
	}

	flashLoan, err := integration.NewFlashLoanLiquidation(client, config.FlashLoanContractAddress.Hex())
	if err != nil {
		return fmt.Errorf("failed to load flash loan contract: %w", err)
	}

	// Verify we're the owner
	owner, err := flashLoan.GetOwner(ctx)
	if err != nil {
		return fmt.Errorf("failed to get contract owner: %w", err)
	}

	if owner != botAddress {
		log.Printf("⚠️  Bot address is not contract owner. Owner: %s, Bot: %s", owner.Hex(), botAddress.Hex())
		return fmt.Errorf("not contract owner")
	}

	log.Printf("Checking contract balances for withdrawal...")

	// Get tokens to check
	tokensToCheck := config.WithdrawTokens
	if len(tokensToCheck) == 0 {
		// Default: check collateral asset (usually aToken)
		tokensToCheck = []common.Address{config.DefaultCollateralAsset}
	}

	totalWithdrawn := big.NewInt(0)

	for _, tokenAddr := range tokensToCheck {
		balance, err := flashLoan.GetContractBalance(ctx, tokenAddr)
		if err != nil {
			log.Printf("  ⚠️  Could not check balance for %s: %v", tokenAddr.Hex(), err)
			continue
		}

		if balance.Cmp(big.NewInt(0)) == 0 {
			continue // No balance
		}

		// Check if above minimum withdrawal amount
		if balance.Cmp(config.MinWithdrawAmount) < 0 {
			log.Printf("  %s: Balance %s below minimum (%s), skipping", tokenAddr.Hex(), balance.String(), config.MinWithdrawAmount.String())
			continue
		}

		log.Printf("  %s: Balance %s - Withdrawing...", tokenAddr.Hex(), balance.String())

		// Update nonce
		nonce, err := client.PendingNonceAt(ctx, botAddress)
		if err != nil {
			return fmt.Errorf("failed to get nonce: %w", err)
		}
		auth.Nonce = big.NewInt(int64(nonce))

		// Try to withdraw as aToken first (redeems to underlying)
		txHash, err := flashLoan.WithdrawAToken(ctx, auth, tokenAddr)
		if err != nil {
			// If aToken withdrawal fails, try regular withdraw
			log.Printf("    aToken withdrawal failed, trying regular withdraw...")
			txHash, err = flashLoan.Withdraw(ctx, auth, tokenAddr)
			if err != nil {
				log.Printf("    ✗ Withdrawal failed: %v", err)
				continue
			}
		}

		log.Printf("    ✓ Withdrawal tx sent: %s", txHash.Hex())
		log.Printf("    Waiting for confirmation...")

		// Wait for transaction
		timeout := time.After(2 * time.Minute)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		var receipt *types.Receipt
		for {
			select {
			case <-timeout:
				log.Printf("    ⚠️  Timeout waiting for receipt")
				goto nextToken
			case <-ticker.C:
				var receiptErr error
				receipt, receiptErr = client.TransactionReceipt(ctx, *txHash)
				if receiptErr == nil {
					goto receiptReceived
				}
			}
		}
	receiptReceived:
		if receipt.Status == 0 {
			log.Printf("    ✗ Transaction reverted")
			continue
		}

		log.Printf("    ✓ Withdrawal confirmed in block %d", receipt.BlockNumber.Uint64())
		totalWithdrawn.Add(totalWithdrawn, balance)
	nextToken:
	}

	if totalWithdrawn.Cmp(big.NewInt(0)) > 0 {
		log.Printf("========================================")
		log.Printf("PROFITS WITHDRAWN! 💰")
		log.Printf("Total withdrawn: %s", totalWithdrawn.String())
		log.Printf("========================================\n")
	} else {
		log.Printf("No profits to withdraw (all balances below minimum or zero)\n")
	}

	return nil
}

// startProfitWithdrawal starts periodic profit withdrawal
func startProfitWithdrawal(
	ctx context.Context,
	client *ethclient.Client,
	config *MonitorConfig,
	auth *bind.TransactOpts,
	botAddress common.Address,
) {
	if !config.EnableAutoWithdraw {
		return
	}

	log.Printf("Starting automatic profit withdrawal (interval: %v)", config.WithdrawInterval)

	ticker := time.NewTicker(config.WithdrawInterval)
	defer ticker.Stop()

	// Initial withdrawal check
	if err := withdrawProfits(ctx, client, config, auth, botAddress); err != nil {
		log.Printf("Initial withdrawal check failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := withdrawProfits(ctx, client, config, auth, botAddress); err != nil {
				log.Printf("Withdrawal failed: %v", err)
			}
		}
	}
}
