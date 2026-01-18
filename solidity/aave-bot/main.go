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
	"sync"
	"syscall"
	"time"

	"aave-bot/contracts"
	"aave-bot/integration"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

// UserPosition represents a tracked user's Aave position
type UserPosition struct {
	Address             common.Address
	LastHealthFactor    *big.Float
	LastCheckedBlock    uint64
	LastCheckedTime     time.Time
	TotalCollateralBase *big.Int
	TotalDebtBase       *big.Int
}

// MonitorConfig holds monitoring parameters
type MonitorConfig struct {
	PoolAddress              common.Address
	PollInterval             time.Duration
	HistoricalBlocksLookback uint64
	HealthFactorThreshold    *big.Float
	EnableAutoLiquidation    bool
	LiquidationProfitThreshold *big.Float
	MaxLiquidationAmount     *big.Int
	DefaultDebtAsset         common.Address
	DefaultCollateralAsset   common.Address
	// Flash loan configuration
	UseFlashLoanLiquidation  bool
	FlashLoanContractAddress common.Address
	// Profit withdrawal configuration
	EnableAutoWithdraw       bool
	WithdrawInterval         time.Duration
	MinWithdrawAmount        *big.Int
	WithdrawTokens          []common.Address // Token addresses to check for withdrawal
}

// LiquidationOpportunity represents a liquidation chance
type LiquidationOpportunity struct {
	User                 common.Address
	HealthFactor         *big.Float
	TotalCollateralBase  *big.Int
	TotalDebtBase        *big.Int
	AvailableBorrowsBase *big.Int
	LiquidationThreshold *big.Int
	Timestamp            time.Time
	BlockNumber          uint64
}

// ProfitCalculation represents profit analysis for a liquidation
type ProfitCalculation struct {
	DebtToCover           *big.Int
	LiquidationBonusPercent *big.Float
	ExpectedCollateral    *big.Int
	EstimatedGasCost      *big.Int
	NetProfit             *big.Int
	ProfitPercentage      *big.Float
	IsProfitable          bool
}

// convertRayToFloat converts Aave's ray format (1e27) to float
func convertRayToFloat(ray *big.Int) *big.Float {
	// 1e27 = 1.0 health factor
	rayDivisor := new(big.Float).SetInt(new(big.Int).Exp(
		big.NewInt(10),
		big.NewInt(27),
		nil,
	))
	return new(big.Float).Quo(
		new(big.Float).SetInt(ray),
		rayDivisor,
	)
}

// loadConfig loads configuration from environment
func loadConfig() (*MonitorConfig, error) {
	poolAddr := os.Getenv("POOL_ADDRESS")
	if poolAddr == "" {
		return nil, fmt.Errorf("POOL_ADDRESS not set in .env")
	}

	pollInterval := 5 // default
	if val := os.Getenv("POLL_INTERVAL"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			pollInterval = parsed
		}
	}

	lookback := uint64(1000) // default
	if val := os.Getenv("HISTORICAL_BLOCKS_LOOKBACK"); val != "" {
		if parsed, err := strconv.ParseUint(val, 10, 64); err == nil {
			lookback = parsed
		}
	}

	// Liquidation settings
	enableAutoLiquidation := false
	if val := os.Getenv("ENABLE_AUTO_LIQUIDATION"); val == "true" {
		enableAutoLiquidation = true
	}

	profitThreshold := big.NewFloat(0.01) // 1% default
	if val := os.Getenv("LIQUIDATION_PROFIT_THRESHOLD"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			profitThreshold = big.NewFloat(parsed)
		}
	}

	maxLiqAmount, ok := new(big.Int).SetString(os.Getenv("MAX_LIQUIDATION_AMOUNT"), 10)
	if !ok || maxLiqAmount == nil {
		maxLiqAmount = big.NewInt(1e18) // 1 token default
	}

	debtAsset := os.Getenv("DEFAULT_DEBT_ASSET")
	if debtAsset == "" {
		debtAsset = "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2" // WETH default
	}

	collateralAsset := os.Getenv("DEFAULT_COLLATERAL_ASSET")
	if collateralAsset == "" {
		collateralAsset = "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2" // WETH default
	}

	// Flash loan configuration
	useFlashLoan := false
	if val := os.Getenv("USE_FLASHLOAN_LIQUIDATION"); val == "true" {
		useFlashLoan = true
	}

	flashLoanContract := common.Address{}
	if flashLoanAddr := os.Getenv("FLASHLOAN_CONTRACT_ADDRESS"); flashLoanAddr != "" {
		flashLoanContract = common.HexToAddress(flashLoanAddr)
		if useFlashLoan && flashLoanContract == (common.Address{}) {
			return nil, fmt.Errorf("USE_FLASHLOAN_LIQUIDATION=true but FLASHLOAN_CONTRACT_ADDRESS not set")
		}
	}

	// Profit withdrawal configuration
	enableAutoWithdraw := false
	if val := os.Getenv("ENABLE_AUTO_WITHDRAW"); val == "true" {
		enableAutoWithdraw = true
	}

	withdrawInterval := 3600 // Default: 1 hour
	if val := os.Getenv("WITHDRAW_INTERVAL"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			withdrawInterval = parsed
		}
	}

	minWithdrawAmount, ok := new(big.Int).SetString(os.Getenv("MIN_WITHDRAW_AMOUNT"), 10)
	if !ok || minWithdrawAmount == nil {
		minWithdrawAmount = big.NewInt(0) // Withdraw any amount by default
	}

	// Parse withdraw tokens (comma-separated addresses)
	var withdrawTokens []common.Address
	if tokensStr := os.Getenv("WITHDRAW_TOKENS"); tokensStr != "" {
		tokens := strings.Split(tokensStr, ",")
		for _, token := range tokens {
			token = strings.TrimSpace(token)
			if token != "" {
				withdrawTokens = append(withdrawTokens, common.HexToAddress(token))
			}
		}
	}

	return &MonitorConfig{
		PoolAddress:                common.HexToAddress(poolAddr),
		PollInterval:               time.Duration(pollInterval) * time.Second,
		HistoricalBlocksLookback:   lookback,
		HealthFactorThreshold:      big.NewFloat(1.0),
		EnableAutoLiquidation:      enableAutoLiquidation,
		LiquidationProfitThreshold: profitThreshold,
		MaxLiquidationAmount:       maxLiqAmount,
		DefaultDebtAsset:           common.HexToAddress(debtAsset),
		DefaultCollateralAsset:     common.HexToAddress(collateralAsset),
		UseFlashLoanLiquidation:    useFlashLoan,
		FlashLoanContractAddress:  flashLoanContract,
		EnableAutoWithdraw:         enableAutoWithdraw,
		WithdrawInterval:           time.Duration(withdrawInterval) * time.Second,
		MinWithdrawAmount:          minWithdrawAmount,
		WithdrawTokens:            withdrawTokens,
	}, nil
}

// logLiquidationOpportunity logs and prepares liquidation data
func logLiquidationOpportunity(opp *LiquidationOpportunity) {
	log.Printf("========================================")
	log.Printf("LIQUIDATION OPPORTUNITY DETECTED!")
	log.Printf("========================================")
	log.Printf("User Address: %s", opp.User.Hex())
	log.Printf("Health Factor: %.6f (Below 1.0 threshold!)", opp.HealthFactor)
	log.Printf("Total Collateral: %s", opp.TotalCollateralBase.String())
	log.Printf("Total Debt: %s", opp.TotalDebtBase.String())
	log.Printf("Available Borrows: %s", opp.AvailableBorrowsBase.String())
	log.Printf("Liquidation Threshold: %s", opp.LiquidationThreshold.String())
	log.Printf("Block Number: %d", opp.BlockNumber)
	log.Printf("Timestamp: %s", opp.Timestamp.Format(time.RFC3339))
	log.Printf("========================================")

	// Prepare liquidation transaction data (for future use)
	log.Printf("Transaction preparation data:")
	log.Printf("  - Liquidator should call: liquidationCall()")
	log.Printf("  - User to liquidate: %s", opp.User.Hex())
	log.Printf("  - Max debt to cover: ~50%% of total debt = %s",
		new(big.Int).Div(opp.TotalDebtBase, big.NewInt(2)).String())
	log.Printf("========================================\n")
}

// calculateProfitability calculates if a liquidation is profitable
func calculateProfitability(
	ctx context.Context,
	pool *contracts.IPool,
	config *MonitorConfig,
	opp *LiquidationOpportunity,
	gasPrice *big.Int,
) (*ProfitCalculation, error) {
	// Calculate debt to cover (50% of total debt, capped by max liquidation amount)
	debtToCover := new(big.Int).Div(opp.TotalDebtBase, big.NewInt(2))
	if debtToCover.Cmp(config.MaxLiquidationAmount) > 0 {
		debtToCover = new(big.Int).Set(config.MaxLiquidationAmount)
	}

	// Get liquidation bonus from Aave reserve configuration
	// Aave V3 typically has 5% liquidation bonus (500 basis points = 0.05)
	// The bonus is stored in the reserve configuration
	reserveData, err := pool.GetReserveData(&bind.CallOpts{Context: ctx}, config.DefaultCollateralAsset)
	if err != nil {
		// If we can't get reserve data, use default 5% bonus
		log.Printf("Warning: Could not fetch reserve data, using default 5%% liquidation bonus")
	}

	// Extract liquidation bonus from configuration
	// Configuration bitmap structure: liquidation bonus is at bits 32-47 (in basis points)
	configuration := reserveData.Configuration
	liquidationBonusBps := new(big.Int).And(
		new(big.Int).Rsh(configuration, 32),
		big.NewInt(0xFFFF), // 16 bits mask
	)

	// Convert basis points to percentage (10000 bps = 100% = 1.0)
	liquidationBonusPercent := new(big.Float).Quo(
		new(big.Float).SetInt(liquidationBonusBps),
		big.NewFloat(10000),
	)

	// If bonus is 0 or unreasonably high, use default 5%
	bonusFloat, _ := liquidationBonusPercent.Float64()
	if bonusFloat == 0 || bonusFloat > 0.2 {
		liquidationBonusPercent = big.NewFloat(0.05) // 5% default
		log.Printf("Using default liquidation bonus: 5%%")
	}

	// Calculate expected collateral received
	// Formula: collateral = debtToCover * (1 + liquidationBonus)
	bonusPlusOne := new(big.Float).Add(big.NewFloat(1.0), liquidationBonusPercent)
	expectedCollateralFloat := new(big.Float).Mul(
		new(big.Float).SetInt(debtToCover),
		bonusPlusOne,
	)
	expectedCollateral, _ := expectedCollateralFloat.Int(nil)

	// Estimate gas cost
	// Liquidation typically costs 300,000-500,000 gas
	// We'll estimate 400,000 gas as a conservative estimate
	estimatedGas := big.NewInt(400000)
	estimatedGasCost := new(big.Int).Mul(estimatedGas, gasPrice)

	// Calculate net profit
	// Profit = (expectedCollateral - debtToCover) - gasCost
	profit := new(big.Int).Sub(expectedCollateral, debtToCover)
	netProfit := new(big.Int).Sub(profit, estimatedGasCost)

	// Calculate profit percentage relative to debt covered
	netProfitFloat := new(big.Float).SetInt(netProfit)
	debtToCoverFloat := new(big.Float).SetInt(debtToCover)
	profitPercentage := new(big.Float).Quo(netProfitFloat, debtToCoverFloat)

	// Check if profitable
	isProfitable := profitPercentage.Cmp(config.LiquidationProfitThreshold) >= 0

	return &ProfitCalculation{
		DebtToCover:             debtToCover,
		LiquidationBonusPercent: liquidationBonusPercent,
		ExpectedCollateral:      expectedCollateral,
		EstimatedGasCost:        estimatedGasCost,
		NetProfit:               netProfit,
		ProfitPercentage:        profitPercentage,
		IsProfitable:            isProfitable,
	}, nil
}

// logProfitability logs profitability analysis
func logProfitability(profit *ProfitCalculation) {
	bonusPercent, _ := profit.LiquidationBonusPercent.Float64()
	profitPercent, _ := profit.ProfitPercentage.Float64()

	log.Printf("========================================")
	log.Printf("PROFIT ANALYSIS")
	log.Printf("========================================")
	log.Printf("Debt to Cover: %s", profit.DebtToCover.String())
	log.Printf("Liquidation Bonus: %.2f%%", bonusPercent*100)
	log.Printf("Expected Collateral: %s", profit.ExpectedCollateral.String())
	log.Printf("Estimated Gas Cost: %s", profit.EstimatedGasCost.String())
	log.Printf("Net Profit: %s", profit.NetProfit.String())
	log.Printf("Profit Percentage: %.4f%% ", profitPercent*100)

	if profit.IsProfitable {
		log.Printf("Status: ✓ PROFITABLE - Proceeding with liquidation")
	} else {
		log.Printf("Status: ✗ NOT PROFITABLE - Skipping liquidation")
	}
	log.Printf("========================================\n")
}

// executeLiquidation executes a liquidation transaction
// Uses flash loan if configured, otherwise uses direct liquidation
func executeLiquidation(
	ctx context.Context,
	client *ethclient.Client,
	pool *contracts.IPool,
	debtToken *contracts.ERC20,
	config *MonitorConfig,
	opp *LiquidationOpportunity,
	auth *bind.TransactOpts,
	botAddress common.Address,
) error {
	// Use flash loan liquidation if configured
	if config.UseFlashLoanLiquidation && config.FlashLoanContractAddress != (common.Address{}) {
		return executeFlashLoanLiquidation(ctx, client, config, opp, auth, botAddress)
	}

	// Otherwise use direct liquidation (original method)
	return executeDirectLiquidation(ctx, client, pool, debtToken, config, opp, auth, botAddress)
}

// executeFlashLoanLiquidation executes liquidation using flash loan contract
func executeFlashLoanLiquidation(
	ctx context.Context,
	client *ethclient.Client,
	config *MonitorConfig,
	opp *LiquidationOpportunity,
	auth *bind.TransactOpts,
	botAddress common.Address,
) error {
	log.Printf("  Using Flash Loan Liquidation Contract...")
	log.Printf("  Contract: %s", config.FlashLoanContractAddress.Hex())

	// Load flash loan contract
	flashLoan, err := integration.NewFlashLoanLiquidation(client, config.FlashLoanContractAddress.Hex())
	if err != nil {
		return fmt.Errorf("failed to load flash loan contract: %w", err)
	}

	// Calculate debt to cover (same logic as direct liquidation)
	debtToCover := new(big.Int).Div(opp.TotalDebtBase, big.NewInt(2)) // 50% of debt
	if debtToCover.Cmp(config.MaxLiquidationAmount) > 0 {
		debtToCover = config.MaxLiquidationAmount
	}

	log.Printf("  Debt to cover: %s", debtToCover.String())
	log.Printf("  Debt asset: %s", config.DefaultDebtAsset.Hex())
	log.Printf("  Collateral asset: %s", config.DefaultCollateralAsset.Hex())

	// Update nonce
	nonce, err := client.PendingNonceAt(ctx, botAddress)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %w", err)
	}
	auth.Nonce = big.NewInt(int64(nonce))

	// Execute flash loan liquidation
	var txHash *common.Hash
	txHash, err = flashLoan.ExecuteLiquidation(
		ctx,
		auth,
		config.DefaultDebtAsset,
		debtToCover,
		opp.User,
		config.DefaultCollateralAsset,
	)
	if err != nil {
		return fmt.Errorf("failed to execute flash loan liquidation: %w", err)
	}

	log.Printf("  Flash loan liquidation tx sent: %s", txHash.Hex())
	log.Printf("  Waiting for confirmation...")

	// Wait for transaction receipt by polling
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	var receipt *types.Receipt
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for transaction receipt")
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
		return fmt.Errorf("flash loan liquidation transaction reverted")
	}

	log.Printf("========================================")
	log.Printf("FLASH LOAN LIQUIDATION SUCCESSFUL! 🎉")
	log.Printf("========================================")
	log.Printf("Transaction hash: %s", txHash.Hex())
	log.Printf("Block number: %d", receipt.BlockNumber.Uint64())
	log.Printf("Gas used: %d", receipt.GasUsed)
	log.Printf("========================================\n")

	return nil
}

// executeDirectLiquidation executes direct liquidation (original method)
func executeDirectLiquidation(
	ctx context.Context,
	client *ethclient.Client,
	pool *contracts.IPool,
	debtToken *contracts.ERC20,
	config *MonitorConfig,
	opp *LiquidationOpportunity,
	auth *bind.TransactOpts,
	botAddress common.Address,
) error {
	// Calculate debt to cover (50% of total debt, capped by max liquidation amount)
	debtToCover := new(big.Int).Div(opp.TotalDebtBase, big.NewInt(2))
	if debtToCover.Cmp(config.MaxLiquidationAmount) > 0 {
		debtToCover = new(big.Int).Set(config.MaxLiquidationAmount)
	}

	log.Printf("Preparing liquidation transaction...")
	log.Printf("  Debt to cover: %s", debtToCover.String())
	log.Printf("  Debt asset: %s", config.DefaultDebtAsset.Hex())
	log.Printf("  Collateral asset: %s", config.DefaultCollateralAsset.Hex())

	// Check bot's balance of debt token
	balance, err := debtToken.BalanceOf(&bind.CallOpts{Context: ctx}, botAddress)
	if err != nil {
		return fmt.Errorf("failed to check token balance: %w", err)
	}

	if balance.Cmp(debtToCover) < 0 {
		return fmt.Errorf("insufficient balance: have %s, need %s", balance.String(), debtToCover.String())
	}

	log.Printf("  Bot balance: %s ✓", balance.String())

	// Check allowance
	allowance, err := debtToken.Allowance(&bind.CallOpts{Context: ctx}, botAddress, config.PoolAddress)
	if err != nil {
		return fmt.Errorf("failed to check allowance: %w", err)
	}

	// Approve if needed
	if allowance.Cmp(debtToCover) < 0 {
		log.Printf("  Approving Pool contract to spend tokens...")

		// Approve max uint256 to avoid future approvals
		maxApproval := new(big.Int)
		maxApproval.SetString("115792089237316195423570985008687907853269984665640564039457584007913129639935", 10) // max uint256

		approveTx, err := debtToken.Approve(auth, config.PoolAddress, maxApproval)
		if err != nil {
			return fmt.Errorf("failed to approve: %w", err)
		}

		log.Printf("  Approval tx sent: %s", approveTx.Hash().Hex())
		log.Printf("  Waiting for approval confirmation...")

		receipt, err := bind.WaitMined(ctx, client, approveTx)
		if err != nil {
			return fmt.Errorf("approval transaction failed: %w", err)
		}

		if receipt.Status == 0 {
			return fmt.Errorf("approval transaction reverted")
		}

		log.Printf("  Approval confirmed in block %d ✓", receipt.BlockNumber.Uint64())
	} else {
		log.Printf("  Sufficient allowance already granted ✓")
	}

	// Execute liquidation
	log.Printf("  Executing liquidation...")

	// Update nonce for new transaction
	nonce, err := client.PendingNonceAt(ctx, botAddress)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %w", err)
	}
	auth.Nonce = big.NewInt(int64(nonce))

	liqTx, err := pool.LiquidationCall(
		auth,
		config.DefaultCollateralAsset,
		config.DefaultDebtAsset,
		opp.User,
		debtToCover,
		false, // receive underlying asset, not aToken
	)
	if err != nil {
		return fmt.Errorf("failed to call liquidation: %w", err)
	}

	log.Printf("  Liquidation tx sent: %s", liqTx.Hash().Hex())
	log.Printf("  Waiting for confirmation...")

	receipt, err := bind.WaitMined(ctx, client, liqTx)
	if err != nil {
		return fmt.Errorf("liquidation transaction failed: %w", err)
	}

	if receipt.Status == 0 {
		return fmt.Errorf("liquidation transaction reverted")
	}

	log.Printf("========================================")
	log.Printf("DIRECT LIQUIDATION SUCCESSFUL! 🎉")
	log.Printf("========================================")
	log.Printf("Transaction hash: %s", liqTx.Hash().Hex())
	log.Printf("Block number: %d", receipt.BlockNumber.Uint64())
	log.Printf("Gas used: %d", receipt.GasUsed)
	log.Printf("========================================\n")

	return nil
}

// scanHistoricalEvents finds addresses from past events
func scanHistoricalEvents(
	ctx context.Context,
	client *ethclient.Client,
	config *MonitorConfig,
	addressMap *sync.Map,
) error {
	currentBlock, err := client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block: %w", err)
	}

	fromBlock := currentBlock - config.HistoricalBlocksLookback
	if fromBlock < 0 {
		fromBlock = 0
	}

	log.Printf("Scanning historical events from block %d to %d", fromBlock, currentBlock)
	log.Printf("Pool address: %s", config.PoolAddress.Hex())

	// Event signatures for Aave Pool events
	supplyTopic := crypto.Keccak256Hash([]byte("Supply(address,address,address,uint256,uint16)"))
	borrowTopic := crypto.Keccak256Hash([]byte("Borrow(address,address,address,uint256,uint8,uint256,uint16)"))
	withdrawTopic := crypto.Keccak256Hash([]byte("Withdraw(address,address,address,uint256)"))
	repayTopic := crypto.Keccak256Hash([]byte("Repay(address,address,address,uint256,bool)"))

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(currentBlock)),
		Addresses: []common.Address{config.PoolAddress},
		Topics: [][]common.Hash{
			{supplyTopic, borrowTopic, withdrawTopic, repayTopic},
		},
	}

	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		log.Printf("⚠️  FilterLogs error: %v", err)
		return fmt.Errorf("failed to filter logs: %w", err)
	}

	log.Printf("Found %d event logs in historical scan", len(logs))

	uniqueAddresses := make(map[common.Address]bool)

	for _, vLog := range logs {
		var userAddr common.Address

		if vLog.Topics[0] == supplyTopic || vLog.Topics[0] == borrowTopic {
			// User is in the data section (2nd parameter)
			if len(vLog.Data) >= 32 {
				userAddr = common.BytesToAddress(vLog.Data[0:32])
			}
		} else if vLog.Topics[0] == withdrawTopic || vLog.Topics[0] == repayTopic {
			// User is indexed (2nd topic)
			if len(vLog.Topics) >= 2 {
				userAddr = common.BytesToAddress(vLog.Topics[1].Bytes())
			}
		}

		if userAddr != (common.Address{}) {
			uniqueAddresses[userAddr] = true
		}
	}

	// Add to monitoring map
	for addr := range uniqueAddresses {
		addressMap.Store(addr, &UserPosition{
			Address:         addr,
			LastCheckedTime: time.Now(),
		})
	}

	log.Printf("Found %d unique addresses with Aave positions", len(uniqueAddresses))
	
	if len(uniqueAddresses) == 0 {
		log.Printf("⚠️  No addresses found. This could mean:")
		log.Printf("   - No Aave activity in the last %d blocks", config.HistoricalBlocksLookback)
		log.Printf("   - Try increasing HISTORICAL_BLOCKS_LOOKBACK in .env")
		log.Printf("   - Or wait for real-time event discovery to find new positions")
	}
	
	return nil
}

// subscribeToNewEvents listens for real-time events
func subscribeToNewEvents(
	ctx context.Context,
	client *ethclient.Client,
	config *MonitorConfig,
	addressMap *sync.Map,
) error {
	log.Println("Starting real-time event polling...")
	log.Printf("Polling every 10 seconds for new Aave events...")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	lastScannedBlock, err := client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get starting block: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			currentBlock, err := client.BlockNumber(ctx)
			if err != nil {
				log.Printf("Error getting current block: %v", err)
				continue
			}

			if currentBlock <= lastScannedBlock {
				continue
			}

			// Scan new blocks
			supplyTopic := crypto.Keccak256Hash([]byte("Supply(address,address,address,uint256,uint16)"))
			borrowTopic := crypto.Keccak256Hash([]byte("Borrow(address,address,address,uint256,uint8,uint256,uint16)"))
			withdrawTopic := crypto.Keccak256Hash([]byte("Withdraw(address,address,address,uint256)"))
			repayTopic := crypto.Keccak256Hash([]byte("Repay(address,address,address,uint256,bool)"))

			query := ethereum.FilterQuery{
				FromBlock: big.NewInt(int64(lastScannedBlock + 1)),
				ToBlock:   big.NewInt(int64(currentBlock)),
				Addresses: []common.Address{config.PoolAddress},
				Topics: [][]common.Hash{
					{supplyTopic, borrowTopic, withdrawTopic, repayTopic},
				},
			}

			logs, err := client.FilterLogs(ctx, query)
			if err != nil {
				log.Printf("Error filtering logs: %v", err)
				continue
			}

			if len(logs) > 0 {
				log.Printf("📊 Discovered %d new event(s) in blocks %d-%d", len(logs), lastScannedBlock+1, currentBlock)
			}

			for _, vLog := range logs {
				var userAddr common.Address

				if vLog.Topics[0] == supplyTopic || vLog.Topics[0] == borrowTopic {
					if len(vLog.Data) >= 32 {
						userAddr = common.BytesToAddress(vLog.Data[0:32])
					}
				} else if vLog.Topics[0] == withdrawTopic || vLog.Topics[0] == repayTopic {
					if len(vLog.Topics) >= 2 {
						userAddr = common.BytesToAddress(vLog.Topics[1].Bytes())
					}
				}

				if userAddr != (common.Address{}) {
					if _, exists := addressMap.Load(userAddr); !exists {
						log.Printf("  ➕ New address discovered: %s", userAddr.Hex())
						addressMap.Store(userAddr, &UserPosition{
							Address:         userAddr,
							LastCheckedTime: time.Now(),
						})
					}
				}
			}

			lastScannedBlock = currentBlock
		}
	}
}

// monitorHealthFactors continuously checks health factors
func monitorHealthFactors(
	ctx context.Context,
	client *ethclient.Client,
	pool *contracts.IPool,
	debtToken *contracts.ERC20,
	config *MonitorConfig,
	addressMap *sync.Map,
	auth *bind.TransactOpts,
	botAddress common.Address,
) error {
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	log.Printf("Starting health factor monitoring (polling every %v)", config.PollInterval)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			currentBlock, err := client.BlockNumber(ctx)
			if err != nil {
				log.Printf("Error getting current block: %v", err)
				continue
			}

			// Count addresses being monitored
			count := 0
			addressMap.Range(func(key, value interface{}) bool {
				count++
				return true
			})

			if count == 0 {
				log.Printf("No addresses to monitor yet...")
				continue
			}

			log.Printf("Checking health factors for %d addresses...", count)

			// Check each address
			addressMap.Range(func(key, value interface{}) bool {
				addr := key.(common.Address)
				position := value.(*UserPosition)

				// Call Pool contract
				accountData, err := pool.GetUserAccountData(&bind.CallOpts{Context: ctx}, addr)
				if err != nil {
					log.Printf("Error getting account data for %s: %v", addr.Hex(), err)
					return true // continue iteration
				}

				// Skip if no debt (nothing to liquidate)
				if accountData.TotalDebtBase.Cmp(big.NewInt(0)) == 0 {
					return true
				}

				// Convert health factor to float
				healthFactorFloat := convertRayToFloat(accountData.HealthFactor)

				// Update position
				position.LastHealthFactor = healthFactorFloat
				position.LastCheckedBlock = currentBlock
				position.LastCheckedTime = time.Now()
				position.TotalCollateralBase = accountData.TotalCollateralBase
				position.TotalDebtBase = accountData.TotalDebtBase

				// Check if below threshold
				if healthFactorFloat.Cmp(config.HealthFactorThreshold) < 0 {
					// LIQUIDATION OPPORTUNITY!
					opp := &LiquidationOpportunity{
						User:                 addr,
						HealthFactor:         healthFactorFloat,
						TotalCollateralBase:  accountData.TotalCollateralBase,
						TotalDebtBase:        accountData.TotalDebtBase,
						AvailableBorrowsBase: accountData.AvailableBorrowsBase,
						LiquidationThreshold: accountData.CurrentLiquidationThreshold,
						Timestamp:            time.Now(),
						BlockNumber:          currentBlock,
					}

					logLiquidationOpportunity(opp)

					// Calculate profitability
					gasPrice, err := client.SuggestGasPrice(ctx)
					if err != nil {
						log.Printf("ERROR: Failed to get gas price: %v", err)
						return true // continue to next address
					}

					profitCalc, err := calculateProfitability(ctx, pool, config, opp, gasPrice)
					if err != nil {
						log.Printf("ERROR: Failed to calculate profitability: %v", err)
						return true // continue to next address
					}

					logProfitability(profitCalc)

					// Execute liquidation only if profitable and auto-liquidation is enabled
					if config.EnableAutoLiquidation {
						if profitCalc.IsProfitable {
							log.Printf("Liquidation is profitable. Executing...")
							if err := executeLiquidation(ctx, client, pool, debtToken, config, opp, auth, botAddress); err != nil {
								log.Printf("ERROR: Liquidation failed: %v", err)
							}
						} else {
							log.Printf("Liquidation not profitable. Skipping execution.")
						}
					} else {
						log.Printf("Auto-liquidation disabled. Skipping execution.")
					}
				} else {
					// Optional: Log healthy positions periodically
					if count <= 10 { // Only log if monitoring few addresses
						log.Printf("  %s: HF=%.4f ✓ (Collateral=%s, Debt=%s)",
							addr.Hex()[:10]+"...",
							healthFactorFloat,
							accountData.TotalCollateralBase.String(),
							accountData.TotalDebtBase.String(),
						)
					}
				}

				return true // continue iteration
			})
		}
	}
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("ERROR: Failed to load config: %v", err)
	}

	log.Printf("Configuration loaded:")
	log.Printf("  Pool Address: %s", config.PoolAddress.Hex())
	log.Printf("  Poll Interval: %v", config.PollInterval)
	log.Printf("  Historical Lookback: %d blocks", config.HistoricalBlocksLookback)
	log.Printf("  Auto-Liquidation: %v", config.EnableAutoLiquidation)
	log.Printf("  Flash Loan Liquidation: %v", config.UseFlashLoanLiquidation)
	if config.UseFlashLoanLiquidation {
		log.Printf("  Flash Loan Contract: %s", config.FlashLoanContractAddress.Hex())
	}
	if config.EnableAutoLiquidation {
		log.Printf("  Debt Asset: %s", config.DefaultDebtAsset.Hex())
		log.Printf("  Collateral Asset: %s", config.DefaultCollateralAsset.Hex())
		log.Printf("  Max Liquidation Amount: %s", config.MaxLiquidationAmount.String())
	}
	if config.EnableAutoWithdraw {
		log.Printf("  Auto-Withdraw: Enabled (interval: %v)", config.WithdrawInterval)
		log.Printf("  Min Withdraw Amount: %s", config.MinWithdrawAmount.String())
	}

	// Connect to Ethereum node
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "http://127.0.0.1:8545"
	}

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("ERROR: Failed to connect to Ethereum node: %v", err)
	}
	defer client.Close()

	log.Printf("Connected to Ethereum node: %s", rpcURL)

	// Setup wallet (for future liquidation transactions)
	privateKeyHex := os.Getenv("PRIVATE_KEY")
	if privateKeyHex == "" {
		log.Fatal("ERROR: PRIVATE_KEY environment variable is not set")
	}

	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		log.Fatalf("ERROR: Failed to parse private key: %v", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("ERROR: Failed to cast public key to ECDSA")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	log.Printf("Bot wallet address: %s", fromAddress.Hex())

	// Create context for operations
	ctx := context.Background()

	// Create Pool contract instance
	pool, err := contracts.NewIPool(config.PoolAddress, client)
	if err != nil {
		log.Fatalf("ERROR: Failed to instantiate Pool contract: %v", err)
	}

	log.Println("Pool contract instantiated successfully")

	// Create ERC20 token instance for debt asset (for liquidations)
	debtToken, err := contracts.NewERC20(config.DefaultDebtAsset, client)
	if err != nil {
		log.Fatalf("ERROR: Failed to instantiate debt token contract: %v", err)
	}

	log.Println("Debt token contract instantiated successfully")

	// Setup transaction auth for liquidations
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		log.Fatalf("ERROR: Failed to get nonce: %v", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatalf("ERROR: Failed to get gas price: %v", err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("ERROR: Failed to get chain ID: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		log.Fatalf("ERROR: Failed to create transactor: %v", err)
	}

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = gasPrice

	// Create address tracking map
	addressMap := &sync.Map{}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nShutting down gracefully...")
		cancel()
	}()

	// Phase 1: Scan historical events
	log.Println("\n========================================")
	log.Println("PHASE 1: Discovering addresses from historical events")
	log.Println("========================================")
	if err := scanHistoricalEvents(ctx, client, config, addressMap); err != nil {
		log.Printf("Warning: Historical scan failed: %v", err)
	}

	// Start concurrent goroutines
	var wg sync.WaitGroup

	// Goroutine 1: Monitor health factors
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("\n========================================")
		log.Println("PHASE 2: Starting health factor monitoring")
		log.Println("========================================")
		if err := monitorHealthFactors(ctx, client, pool, debtToken, config, addressMap, auth, fromAddress); err != nil {
			if err != context.Canceled {
				log.Printf("Health factor monitoring error: %v", err)
			}
		}
	}()

	// Goroutine 2: Listen for new events
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("\n========================================")
		log.Println("PHASE 3: Starting real-time event discovery")
		log.Println("========================================")
		if err := subscribeToNewEvents(ctx, client, config, addressMap); err != nil {
			if err != context.Canceled {
				log.Printf("Event subscription error: %v", err)
			}
		}
	}()

	// Goroutine 3: Automatic profit withdrawal (if enabled)
	if config.EnableAutoWithdraw && config.UseFlashLoanLiquidation {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Println("\n========================================")
			log.Println("PHASE 4: Starting automatic profit withdrawal")
			log.Println("========================================")
			startProfitWithdrawal(ctx, client, config, auth, fromAddress)
		}()
	}

	fmt.Println("\n✓ Bot is live and monitoring...")
	fmt.Println("Press Ctrl+C to stop\n")

	// Wait for shutdown
	wg.Wait()
	log.Println("Bot stopped.")
}
