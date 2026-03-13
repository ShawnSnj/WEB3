package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"uniswap-bot/contracts"
	"uniswap-bot/integration"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

// TokenPair represents a pair of tokens to monitor
type TokenPair struct {
	Token0 common.Address
	Token1 common.Address
	Name   string
}

// PriceData represents current price information
type PriceData struct {
	Token0Amount *big.Int
	Token1Amount *big.Int
	Price        *big.Float // Price of token0 in terms of token1
	Timestamp    time.Time
	BlockNumber  uint64
}

// SwapOpportunity represents a profitable swap opportunity
type SwapOpportunity struct {
	Pair          TokenPair
	Direction     string // "token0_to_token1" or "token1_to_token0"
	AmountIn      *big.Int
	AmountOut     *big.Int
	ExpectedOut   *big.Int
	Profit        *big.Int
	ProfitPercent *big.Float
	Price         *big.Float
}

// BotConfig holds bot configuration
type BotConfig struct {
	RPCURL              string
	PrivateKey          *ecdsa.PrivateKey
	WalletAddress       common.Address
	UniswapV2Router     common.Address
	PollInterval        time.Duration
	TokenPairs          []TokenPair
	MinProfitPercentage *big.Float
	MaxSlippageBPS      uint64
	SwapAmount          *big.Int
	GasPriceMultiplier  *big.Float
	EnableAutoSwap      bool
	MinSwapAmount       *big.Int
	MaxSwapAmount       *big.Int
	// Flash loan configuration
	UseFlashLoan        bool
	FlashLoanContract   common.Address
	MinFlashLoanAmount  *big.Int
	MaxFlashLoanAmount  *big.Int
}

// loadConfig loads configuration from environment
func loadConfig() (*BotConfig, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// RPC URL
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		return nil, fmt.Errorf("RPC_URL not set")
	}

	// Private key
	privateKeyHex := os.Getenv("PRIVATE_KEY")
	if privateKeyHex == "" {
		return nil, fmt.Errorf("PRIVATE_KEY not set")
	}
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to get public key")
	}
	walletAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	// Uniswap Router
	routerAddr := os.Getenv("UNISWAP_V2_ROUTER")
	if routerAddr == "" {
		routerAddr = "0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D" // Mainnet default
	}

	// Poll interval
	pollInterval := 5
	if val := os.Getenv("POLL_INTERVAL"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			pollInterval = parsed
		}
	}

	// Token pairs
	var tokenPairs []TokenPair
	if pairsStr := os.Getenv("MONITOR_PAIRS"); pairsStr != "" {
		pairs := strings.Split(pairsStr, ",")
		for i := 0; i < len(pairs)-1; i += 2 {
			token0 := common.HexToAddress(strings.TrimSpace(pairs[i]))
			token1 := common.HexToAddress(strings.TrimSpace(pairs[i+1]))
			tokenPairs = append(tokenPairs, TokenPair{
				Token0: token0,
				Token1: token1,
				Name:   fmt.Sprintf("%s/%s", token0.Hex()[:8], token1.Hex()[:8]),
			})
		}
	}

	if len(tokenPairs) == 0 {
		return nil, fmt.Errorf("no token pairs configured (MONITOR_PAIRS)")
	}

	// Min profit percentage
	minProfit := big.NewFloat(0.01) // 1% default
	if val := os.Getenv("MIN_PROFIT_PERCENTAGE"); val != "" {
		if parsed, _, err := big.NewFloat(0).Parse(val, 10); err == nil {
			minProfit = parsed
		}
	}

	// Max slippage (basis points)
	maxSlippageBPS := uint64(50) // 0.5% default
	if val := os.Getenv("MAX_SLIPPAGE_BPS"); val != "" {
		if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
			maxSlippageBPS = parsed
		}
	}

	// Swap amount
	var swapAmount *big.Int
	if val := os.Getenv("SWAP_AMOUNT"); val != "" {
		swapAmount, _ = new(big.Int).SetString(val, 10)
	}

	// Gas price multiplier
	gasMultiplier := big.NewFloat(1.1) // 10% higher default
	if val := os.Getenv("GAS_PRICE_MULTIPLIER"); val != "" {
		if parsed, _, err := big.NewFloat(0).Parse(val, 10); err == nil {
			gasMultiplier = parsed
		}
	}

	// Auto swap
	enableAutoSwap := false
	if os.Getenv("ENABLE_AUTO_SWAP") == "true" {
		enableAutoSwap = true
	}

	// Min/Max swap amounts
	minSwapAmount, _ := new(big.Int).SetString(os.Getenv("MIN_SWAP_AMOUNT"), 10)
	if minSwapAmount == nil {
		minSwapAmount = big.NewInt(1000000000000000) // 0.001 ETH default
	}

	maxSwapAmount, _ := new(big.Int).SetString(os.Getenv("MAX_SWAP_AMOUNT"), 10)

	// Flash loan configuration
	useFlashLoan := false
	if os.Getenv("USE_FLASHLOAN") == "true" {
		useFlashLoan = true
	}

	flashLoanContract := common.Address{}
	if flashLoanAddr := os.Getenv("FLASHLOAN_CONTRACT_ADDRESS"); flashLoanAddr != "" {
		flashLoanContract = common.HexToAddress(flashLoanAddr)
	}

	minFlashLoanAmount, _ := new(big.Int).SetString(os.Getenv("MIN_FLASHLOAN_AMOUNT"), 10)
	if minFlashLoanAmount == nil {
		minFlashLoanAmount = big.NewInt(1000000000000000000) // 1 token default
	}

	maxFlashLoanAmount, _ := new(big.Int).SetString(os.Getenv("MAX_FLASHLOAN_AMOUNT"), 10)

	return &BotConfig{
		RPCURL:              rpcURL,
		PrivateKey:          privateKey,
		WalletAddress:       walletAddress,
		UniswapV2Router:     common.HexToAddress(routerAddr),
		PollInterval:        time.Duration(pollInterval) * time.Second,
		TokenPairs:          tokenPairs,
		MinProfitPercentage: minProfit,
		MaxSlippageBPS:      maxSlippageBPS,
		SwapAmount:          swapAmount,
		GasPriceMultiplier:  gasMultiplier,
		EnableAutoSwap:      enableAutoSwap,
		MinSwapAmount:       minSwapAmount,
		MaxSwapAmount:       maxSwapAmount,
		UseFlashLoan:        useFlashLoan,
		FlashLoanContract:   flashLoanContract,
		MinFlashLoanAmount:  minFlashLoanAmount,
		MaxFlashLoanAmount:  maxFlashLoanAmount,
	}, nil
}

// getPrice gets the current price for a token pair
func getPrice(ctx context.Context, router *contracts.UniswapV2Router, pair TokenPair, amountIn *big.Int, token0Decimals, token1Decimals uint8) (*PriceData, error) {
	path := []common.Address{pair.Token0, pair.Token1}
	
	amounts, err := router.GetAmountsOut(&bind.CallOpts{Context: ctx}, amountIn, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get amounts out (pool may not exist): %w", err)
	}

	if len(amounts) < 2 {
		return nil, fmt.Errorf("invalid amounts returned from router")
	}

	// Check if amounts are zero (no liquidity)
	if amounts[0].Cmp(big.NewInt(0)) == 0 || amounts[1].Cmp(big.NewInt(0)) == 0 {
		return nil, fmt.Errorf("pool has no liquidity (amounts are zero)")
	}

	// Calculate price accounting for different token decimals
	// amounts[0] is in token0's smallest units (10^token0Decimals)
	// amounts[1] is in token1's smallest units (10^token1Decimals)
	// Price = (amounts[1] / 10^token1Decimals) / (amounts[0] / 10^token0Decimals)
	//      = amounts[1] / amounts[0] * 10^(token0Decimals - token1Decimals)
	
	decimalsDiff := int(token0Decimals) - int(token1Decimals)
	decimalsMultiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimalsDiff)), nil))
	
	price := new(big.Float).Quo(
		new(big.Float).SetInt(amounts[1]),
		new(big.Float).SetInt(amounts[0]),
	)
	price.Mul(price, decimalsMultiplier)

	return &PriceData{
		Token0Amount: amounts[0],
		Token1Amount: amounts[1],
		Price:        price,
		Timestamp:    time.Now(),
	}, nil
}

// findSwapOpportunity finds profitable swap opportunities
func findSwapOpportunity(
	ctx context.Context,
	router *contracts.UniswapV2Router,
	token0 *contracts.ERC20,
	token1 *contracts.ERC20,
	pair TokenPair,
	config *BotConfig,
	walletAddress common.Address,
) (*SwapOpportunity, error) {
	// Get wallet balances
	balance0, err := token0.BalanceOf(&bind.CallOpts{Context: ctx}, walletAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get token0 balance: %w", err)
	}

	balance1, err := token1.BalanceOf(&bind.CallOpts{Context: ctx}, walletAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get token1 balance: %w", err)
	}

	// Determine swap amount
	var amountIn *big.Int
	var path []common.Address

	// Try swapping token0 -> token1
	if balance0.Cmp(config.MinSwapAmount) > 0 {
		if config.SwapAmount != nil && config.SwapAmount.Cmp(big.NewInt(0)) > 0 {
			amountIn = config.SwapAmount
			if amountIn.Cmp(balance0) > 0 {
				amountIn = new(big.Int).Set(balance0)
			}
		} else {
			// Use 50% of balance
			amountIn = new(big.Int).Div(balance0, big.NewInt(2))
		}

		if config.MaxSwapAmount != nil && config.MaxSwapAmount.Cmp(big.NewInt(0)) > 0 && amountIn.Cmp(config.MaxSwapAmount) > 0 {
			amountIn = new(big.Int).Set(config.MaxSwapAmount)
		}

		if amountIn.Cmp(config.MinSwapAmount) >= 0 {
			path = []common.Address{pair.Token0, pair.Token1}
			amounts, err := router.GetAmountsOut(&bind.CallOpts{Context: ctx}, amountIn, path)
			if err == nil && len(amounts) >= 2 {
				amountOut := amounts[1]
				
				// Calculate profit (simplified - in real arbitrage, compare with other DEXes)
				// For now, we'll just check if we can get more token1 than we put in token0
				profit := new(big.Int).Sub(amountOut, amountIn) // Simplified profit calc
				profitPercent := new(big.Float).Quo(
					new(big.Float).SetInt(profit),
					new(big.Float).SetInt(amountIn),
				)

				if profitPercent.Cmp(config.MinProfitPercentage) >= 0 {
					price := new(big.Float).Quo(
						new(big.Float).SetInt(amountOut),
						new(big.Float).SetInt(amountIn),
					)

					return &SwapOpportunity{
						Pair:          pair,
						Direction:     "token0_to_token1",
						AmountIn:      amountIn,
						AmountOut:     amountOut,
						ExpectedOut:   amountOut,
						Profit:        profit,
						ProfitPercent:  profitPercent,
						Price:         price,
					}, nil
				}
			}
		}
	}

	// Try swapping token1 -> token0
	if balance1.Cmp(config.MinSwapAmount) > 0 {
		if config.SwapAmount != nil && config.SwapAmount.Cmp(big.NewInt(0)) > 0 {
			amountIn = config.SwapAmount
			if amountIn.Cmp(balance1) > 0 {
				amountIn = new(big.Int).Set(balance1)
			}
		} else {
			amountIn = new(big.Int).Div(balance1, big.NewInt(2))
		}

		if config.MaxSwapAmount != nil && config.MaxSwapAmount.Cmp(big.NewInt(0)) > 0 && amountIn.Cmp(config.MaxSwapAmount) > 0 {
			amountIn = new(big.Int).Set(config.MaxSwapAmount)
		}

		if amountIn.Cmp(config.MinSwapAmount) >= 0 {
			path = []common.Address{pair.Token1, pair.Token0}
			amounts, err := router.GetAmountsOut(&bind.CallOpts{Context: ctx}, amountIn, path)
			if err == nil && len(amounts) >= 2 {
				amountOut := amounts[1]
				profit := new(big.Int).Sub(amountOut, amountIn)
				profitPercent := new(big.Float).Quo(
					new(big.Float).SetInt(profit),
					new(big.Float).SetInt(amountIn),
				)

				if profitPercent.Cmp(config.MinProfitPercentage) >= 0 {
					price := new(big.Float).Quo(
						new(big.Float).SetInt(amountOut),
						new(big.Float).SetInt(amountIn),
					)

					return &SwapOpportunity{
						Pair:          pair,
						Direction:     "token1_to_token0",
						AmountIn:      amountIn,
						AmountOut:     amountOut,
						ExpectedOut:   amountOut,
						Profit:        profit,
						ProfitPercent:  profitPercent,
						Price:         price,
					}, nil
				}
			}
		}
	}

	return nil, nil // No opportunity found
}

// executeSwap executes a swap transaction
func executeSwap(
	ctx context.Context,
	client *ethclient.Client,
	router *contracts.UniswapV2Router,
	token0 *contracts.ERC20,
	token1 *contracts.ERC20,
	opp *SwapOpportunity,
	config *BotConfig,
) error {
	// Prepare transaction auth
	nonce, err := client.PendingNonceAt(ctx, config.WalletAddress)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to get gas price: %w", err)
	}

	// Apply gas price multiplier
	gasPriceFloat := new(big.Float).SetInt(gasPrice)
	gasPriceFloat.Mul(gasPriceFloat, config.GasPriceMultiplier)
	gasPrice, _ = gasPriceFloat.Int(nil)

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(config.PrivateKey, chainID)
	if err != nil {
		return fmt.Errorf("failed to create transactor: %w", err)
	}

	auth.Nonce = big.NewInt(int64(nonce))
	auth.GasPrice = gasPrice
	auth.GasLimit = 300000
	auth.Context = ctx

	// Determine path and token to approve
	var path []common.Address
	var tokenToApprove *contracts.ERC20

	if opp.Direction == "token0_to_token1" {
		path = []common.Address{opp.Pair.Token0, opp.Pair.Token1}
		tokenToApprove = token0
	} else {
		path = []common.Address{opp.Pair.Token1, opp.Pair.Token0}
		tokenToApprove = token1
	}

	// Check and approve if needed
	allowance, err := tokenToApprove.Allowance(&bind.CallOpts{Context: ctx}, config.WalletAddress, config.UniswapV2Router)
	if err != nil {
		return fmt.Errorf("failed to check allowance: %w", err)
	}

	if allowance.Cmp(opp.AmountIn) < 0 {
		log.Printf("Approving router to spend tokens...")
		maxApproval := new(big.Int)
		maxApproval.SetString("115792089237316195423570985008687907853269984665640564039457584007913129639935", 10)

		approveTx, err := tokenToApprove.Approve(auth, config.UniswapV2Router, maxApproval)
		if err != nil {
			return fmt.Errorf("failed to approve: %w", err)
		}

		log.Printf("Approval tx sent: %s", approveTx.Hash().Hex())
		
		// Wait for approval
		receipt, err := bind.WaitMined(ctx, client, approveTx)
		if err != nil {
			return fmt.Errorf("approval transaction failed: %w", err)
		}

		if receipt.Status == 0 {
			return fmt.Errorf("approval transaction reverted")
		}

		log.Printf("Approval confirmed ✓")
		
		// Update nonce for swap
		nonce, err = client.PendingNonceAt(ctx, config.WalletAddress)
		if err != nil {
			return fmt.Errorf("failed to get nonce: %w", err)
		}
		auth.Nonce = big.NewInt(int64(nonce))
	}

	// Calculate minimum amount out with slippage
	slippageMultiplier := big.NewInt(10000 - int64(config.MaxSlippageBPS))
	amountOutMin := new(big.Int).Mul(opp.ExpectedOut, slippageMultiplier)
	amountOutMin.Div(amountOutMin, big.NewInt(10000))

	// Deadline (20 minutes from now)
	deadline := big.NewInt(time.Now().Add(20 * time.Minute).Unix())

	log.Printf("Executing swap...")
	log.Printf("  Direction: %s", opp.Direction)
	log.Printf("  Amount in: %s", opp.AmountIn.String())
	log.Printf("  Expected out: %s", opp.ExpectedOut.String())
	log.Printf("  Min out (with slippage): %s", amountOutMin.String())
	log.Printf("  Profit: %s (%.2f%%)", opp.Profit.String(), opp.ProfitPercent)

	// Execute swap
	swapTx, err := router.SwapExactTokensForTokens(
		auth,
		opp.AmountIn,
		amountOutMin,
		path,
		config.WalletAddress,
		deadline,
	)
	if err != nil {
		return fmt.Errorf("failed to execute swap: %w", err)
	}

	log.Printf("Swap tx sent: %s", swapTx.Hash().Hex())
	log.Printf("Waiting for confirmation...")

	receipt, err := bind.WaitMined(ctx, client, swapTx)
	if err != nil {
		return fmt.Errorf("swap transaction failed: %w", err)
	}

	if receipt.Status == 0 {
		return fmt.Errorf("swap transaction reverted")
	}

	log.Printf("========================================")
	log.Printf("SWAP SUCCESSFUL! 🎉")
	log.Printf("========================================")
	log.Printf("Transaction hash: %s", swapTx.Hash().Hex())
	log.Printf("Block number: %d", receipt.BlockNumber.Uint64())
	log.Printf("Gas used: %d", receipt.GasUsed)
	log.Printf("========================================\n")

	return nil
}

// executeFlashLoanArbitrage executes a flash loan arbitrage
func executeFlashLoanArbitrage(
	ctx context.Context,
	client *ethclient.Client,
	opp *SwapOpportunity,
	config *BotConfig,
) error {
	// Load flash loan contract
	flashLoan, err := integration.NewFlashLoanArbitrage(client, config.FlashLoanContract.Hex())
	if err != nil {
		return fmt.Errorf("failed to load flash loan contract: %w", err)
	}

	// Prepare transaction auth
	nonce, err := client.PendingNonceAt(ctx, config.WalletAddress)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to get gas price: %w", err)
	}

	// Apply gas price multiplier
	gasPriceFloat := new(big.Float).SetInt(gasPrice)
	gasPriceFloat.Mul(gasPriceFloat, config.GasPriceMultiplier)
	gasPrice, _ = gasPriceFloat.Int(nil)

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(config.PrivateKey, chainID)
	if err != nil {
		return fmt.Errorf("failed to create transactor: %w", err)
	}

	auth.Nonce = big.NewInt(int64(nonce))
	auth.GasPrice = gasPrice
	auth.GasLimit = 500000 // Flash loans need more gas
	auth.Context = ctx

	// Determine tokens based on direction
	var borrowToken, tokenIn, tokenOut common.Address
	if opp.Direction == "token0_to_token1" {
		borrowToken = opp.Pair.Token0
		tokenIn = opp.Pair.Token0
		tokenOut = opp.Pair.Token1
	} else {
		borrowToken = opp.Pair.Token1
		tokenIn = opp.Pair.Token1
		tokenOut = opp.Pair.Token0
	}

	// Calculate expected profit (accounting for flash loan premium ~0.05%)
	// Premium is approximately 0.05% of the loan amount
	premium := new(big.Int).Div(new(big.Int).Mul(opp.AmountIn, big.NewInt(5)), big.NewInt(10000))
	expectedProfit := new(big.Int).Sub(opp.Profit, premium)
	if expectedProfit.Cmp(big.NewInt(0)) < 0 {
		expectedProfit = big.NewInt(0)
	}

	log.Printf("Executing flash loan arbitrage...")
	log.Printf("  Borrow token: %s", borrowToken.Hex())
	log.Printf("  Borrow amount: %s", opp.AmountIn.String())
	log.Printf("  Token in: %s", tokenIn.Hex())
	log.Printf("  Token out: %s", tokenOut.Hex())
	log.Printf("  Expected profit: %s", expectedProfit.String())

	// Execute flash loan arbitrage
	txHash, err := flashLoan.ExecuteArbitrage(
		ctx,
		auth,
		borrowToken,
		opp.AmountIn,
		tokenIn,
		tokenOut,
		expectedProfit,
	)
	if err != nil {
		return fmt.Errorf("failed to execute flash loan arbitrage: %w", err)
	}

	log.Printf("Flash loan arbitrage tx sent: %s", txHash.Hex())
	log.Printf("Waiting for confirmation...")

	// Wait for transaction
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for transaction")
		case <-ticker.C:
			receipt, err := client.TransactionReceipt(ctx, *txHash)
			if err == nil {
				if receipt.Status == 0 {
					return fmt.Errorf("flash loan arbitrage transaction reverted")
				}
				log.Printf("========================================")
				log.Printf("FLASH LOAN ARBITRAGE SUCCESSFUL! 🎉")
				log.Printf("========================================")
				log.Printf("Transaction hash: %s", txHash.Hex())
				log.Printf("Block number: %d", receipt.BlockNumber.Uint64())
				log.Printf("Gas used: %d", receipt.GasUsed)
				log.Printf("========================================\n")
				return nil
			}
		}
	}
}

// monitorPrices continuously monitors prices and executes swaps
func monitorPrices(ctx context.Context, client *ethclient.Client, config *BotConfig) error {
	// Create router instance
	router, err := contracts.NewUniswapV2Router(config.UniswapV2Router, client)
	if err != nil {
		return fmt.Errorf("failed to create router: %w", err)
	}

	// Create token instances
	tokenInstances := make(map[common.Address]*contracts.ERC20)
	for _, pair := range config.TokenPairs {
		if _, exists := tokenInstances[pair.Token0]; !exists {
			token0, err := contracts.NewERC20(pair.Token0, client)
			if err != nil {
				return fmt.Errorf("failed to create token0: %w", err)
			}
			tokenInstances[pair.Token0] = token0
		}
		if _, exists := tokenInstances[pair.Token1]; !exists {
			token1, err := contracts.NewERC20(pair.Token1, client)
			if err != nil {
				return fmt.Errorf("failed to create token1: %w", err)
			}
			tokenInstances[pair.Token1] = token1
		}
	}

	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	log.Println("Starting price monitoring...")
	log.Printf("Monitoring %d token pair(s)", len(config.TokenPairs))
	log.Printf("Poll interval: %v", config.PollInterval)
	log.Printf("Min profit: %.2f%%", config.MinProfitPercentage)
	log.Printf("Auto-swap: %v", config.EnableAutoSwap)
	if config.UseFlashLoan {
		log.Printf("Flash loan: Enabled (contract: %s)", config.FlashLoanContract.Hex())
	}
	log.Println()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for _, pair := range config.TokenPairs {
				token0 := tokenInstances[pair.Token0]
				token1 := tokenInstances[pair.Token1]

				// Get token decimals
				decimals0, err := token0.Decimals(&bind.CallOpts{Context: ctx})
				if err != nil {
					log.Printf("⚠️  Error getting token0 decimals for %s: %v", pair.Name, err)
					continue
				}

				decimals1, err := token1.Decimals(&bind.CallOpts{Context: ctx})
				if err != nil {
					log.Printf("⚠️  Error getting token1 decimals for %s: %v", pair.Name, err)
					continue
				}

				// Calculate 1 token amount based on decimals
				amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals0)), nil)

				// Get current price
				priceData, err := getPrice(ctx, router, pair, amountIn, decimals0, decimals1)
				if err != nil {
					log.Printf("⚠️  Error getting price for %s: %v", pair.Name, err)
					log.Printf("   This usually means:")
					log.Printf("   - No Uniswap pool exists for this pair on this network")
					log.Printf("   - Pool exists but has no liquidity")
					log.Printf("   - Token addresses are incorrect for this network")
					log.Printf("   - Router address is incorrect")
					continue
				}

				priceFloat, _ := priceData.Price.Float64()
				if priceFloat > 0 {
					log.Printf("[%s] Price: %.6f (1 token0 = %.6f token1)", pair.Name, priceFloat, priceFloat)
				} else {
					log.Printf("⚠️  [%s] Price is 0 - pool may have no liquidity", pair.Name)
					log.Printf("   Debug: token0Amount=%s, token1Amount=%s", priceData.Token0Amount.String(), priceData.Token1Amount.String())
				}

				// Find swap opportunities
				opp, err := findSwapOpportunity(ctx, router, token0, token1, pair, config, config.WalletAddress)
				if err != nil {
					log.Printf("Error finding opportunity for %s: %v", pair.Name, err)
					continue
				}

				if opp != nil {
					profitFloat, _ := opp.ProfitPercent.Float64()
					log.Printf("💰 PROFITABLE OPPORTUNITY FOUND!")
					log.Printf("  Pair: %s", pair.Name)
					log.Printf("  Direction: %s", opp.Direction)
					log.Printf("  Profit: %.2f%%", profitFloat*100)
					log.Printf("  Amount in: %s", opp.AmountIn.String())
					log.Printf("  Expected out: %s", opp.ExpectedOut.String())

					if config.EnableAutoSwap {
						// Use flash loan if enabled and amount is large enough
						if config.UseFlashLoan && config.FlashLoanContract != (common.Address{}) {
							// Check if amount qualifies for flash loan
							if (config.MinFlashLoanAmount == nil || opp.AmountIn.Cmp(config.MinFlashLoanAmount) >= 0) &&
								(config.MaxFlashLoanAmount == nil || config.MaxFlashLoanAmount.Cmp(big.NewInt(0)) == 0 || opp.AmountIn.Cmp(config.MaxFlashLoanAmount) <= 0) {
								if err := executeFlashLoanArbitrage(ctx, client, opp, config); err != nil {
									log.Printf("❌ Flash loan arbitrage failed: %v", err)
									// Fallback to regular swap
									log.Printf("  Falling back to regular swap...")
									if err := executeSwap(ctx, client, router, token0, token1, opp, config); err != nil {
										log.Printf("❌ Swap failed: %v", err)
									}
								}
							} else {
								// Amount doesn't qualify for flash loan, use regular swap
								if err := executeSwap(ctx, client, router, token0, token1, opp, config); err != nil {
									log.Printf("❌ Swap failed: %v", err)
								}
							}
						} else {
							// Regular swap
							if err := executeSwap(ctx, client, router, token0, token1, opp, config); err != nil {
								log.Printf("❌ Swap failed: %v", err)
							}
						}
					} else {
						log.Printf("⚠️  Auto-swap disabled. Set ENABLE_AUTO_SWAP=true to execute")
					}
				}
			}
		}
	}
}

func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("ERROR: Failed to load config: %v", err)
	}

	log.Printf("Configuration loaded:")
	log.Printf("  Wallet: %s", config.WalletAddress.Hex())
	log.Printf("  Router: %s", config.UniswapV2Router.Hex())
	log.Printf("  RPC: %s", config.RPCURL)

	// Connect to Ethereum node
	client, err := ethclient.Dial(config.RPCURL)
	if err != nil {
		log.Fatalf("ERROR: Failed to connect to Ethereum node: %v", err)
	}
	defer client.Close()

	log.Printf("Connected to Ethereum node\n")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nShutting down gracefully...")
		cancel()
	}()

	// Start monitoring
	if err := monitorPrices(ctx, client, config); err != nil {
		if err != context.Canceled {
			log.Printf("Monitoring error: %v", err)
		}
	}

	log.Println("Bot stopped.")
}
