# Flash Loan Integration Guide

This guide explains how to integrate the `uniswapFlashloan` contract with the `uniswap-bot`.

## Overview

The flash loan integration allows the bot to:
- Execute arbitrage without requiring capital upfront
- Leverage larger amounts for better profits
- Automatically repay loans in the same transaction

## Setup

### 1. Deploy Flash Loan Contract

```bash
cd ../uniswapFlashloan
npm install
npx hardhat compile
npx hardhat run scripts/deploy.js --network sepolia
```

Save the deployed contract address.

### 2. Configure Bot

Add to `uniswap-bot/.env`:

```env
# Enable flash loan arbitrage
USE_FLASHLOAN=true
FLASHLOAN_CONTRACT_ADDRESS=0x...  # Your deployed contract address

# Flash loan amount limits
MIN_FLASHLOAN_AMOUNT=1000000000000000000  # 1 token minimum
MAX_FLASHLOAN_AMOUNT=0  # 0 = no limit
```

### 3. Run Bot

```bash
cd uniswap-bot
go run .
```

## How It Works

### Regular Swap Flow

```
1. Bot detects opportunity
2. Checks wallet balance
3. Executes swap if balance sufficient
4. Keeps profit
```

### Flash Loan Flow

```
1. Bot detects opportunity
2. Checks if amount qualifies for flash loan
3. Calls flash loan contract
4. Contract:
   a. Borrows tokens from Aave
   b. Executes swap on Uniswap
   c. Repays loan + premium
   d. Keeps profit in contract
5. Owner withdraws profits later
```

## Configuration Options

| Variable | Description | Default |
|----------|-------------|---------|
| `USE_FLASHLOAN` | Enable flash loan arbitrage | `false` |
| `FLASHLOAN_CONTRACT_ADDRESS` | Deployed contract address | - |
| `MIN_FLASHLOAN_AMOUNT` | Minimum amount for flash loan | `1000000000000000000` (1 token) |
| `MAX_FLASHLOAN_AMOUNT` | Maximum amount for flash loan | `0` (no limit) |

## When to Use Flash Loans

**Use flash loans when:**
- ✅ Opportunity requires more capital than you have
- ✅ Profit margin is high enough to cover premium (~0.05%)
- ✅ Gas costs are acceptable relative to profit

**Use regular swaps when:**
- ✅ You have sufficient balance
- ✅ Amount is small (< 1 token)
- ✅ Gas costs would eat into profit

## Profit Calculation

Flash loan arbitrage profit:

```
Profit = Swap Output - (Loan Amount + Premium + Gas)
```

Where:
- `Premium` ≈ 0.05% of loan amount (Aave fee)
- `Gas` = Transaction gas cost

## Example

### Opportunity Detected

```
Pair: WETH/USDC
Direction: WETH -> USDC
Amount in: 10 ETH
Expected out: 25,000 USDC
Profit: 2.5%
```

### With Flash Loan

```
1. Borrow 10 ETH via flash loan
2. Swap 10 ETH → 25,000 USDC
3. Repay: 10 ETH + 0.005 ETH (premium) = 10.005 ETH
4. Profit: 25,000 USDC - (10.005 ETH * 2,500) = ~12.5 USDC
5. Profit stored in contract
```

### With Regular Swap

```
1. Use 10 ETH from wallet
2. Swap 10 ETH → 25,000 USDC
3. Profit: 25,000 USDC - (10 ETH * 2,500) = 0 USDC
4. (No profit if you already had the tokens)
```

## Withdrawing Profits

Profits from flash loan arbitrage are stored in the contract. To withdraw:

### Option 1: Using Go Script

```bash
cd uniswap-bot
TOKEN_ADDRESS=0x... go run scripts/withdrawProfit.go
```

### Option 2: Using Hardhat Script

```bash
cd ../uniswapFlashloan
TOKEN_ADDRESS=0x... \
FLASHLOAN_CONTRACT_ADDRESS=0x... \
npx hardhat run scripts/withdrawProfit.js --network sepolia
```

## Troubleshooting

### "Flash loan arbitrage failed"

- Check contract has sufficient gas
- Verify token addresses are correct
- Ensure profit covers premium + gas
- Check slippage settings

### "Insufficient profit"

- Flash loan premium (~0.05%) reduces profit
- Increase `MIN_PROFIT_PERCENTAGE` to account for premium
- Or use regular swaps for smaller opportunities

### "Transaction reverted"

- Price may have changed (slippage)
- Pool may not have enough liquidity
- Check contract owner is correct

## Best Practices

1. **Start Small**: Test with small amounts first
2. **Monitor Gas**: Flash loans use more gas (~500k)
3. **Set Limits**: Use `MIN_FLASHLOAN_AMOUNT` to avoid small trades
4. **Withdraw Regularly**: Don't let profits accumulate too much
5. **Test on Testnet**: Always test on Sepolia first

## Summary

Flash loans enable capital-efficient arbitrage by:
- ✅ No upfront capital required
- ✅ Larger trade sizes possible
- ✅ Automatic repayment
- ✅ Profits stored in contract

The bot automatically chooses between flash loans and regular swaps based on:
- Opportunity size
- Available balance
- Configuration limits
