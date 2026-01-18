# Profit Withdrawal Guide

Your bot can now automatically withdraw profits from the flash loan contract to your wallet!

---

## 💰 How It Works

When liquidations are successful:
1. **Profits go to contract** - Collateral (aToken) is received in the flash loan contract
2. **Auto-withdrawal** - Bot periodically checks contract balance and withdraws to your wallet
3. **Manual withdrawal** - You can also withdraw manually anytime

---

## ⚙️ Configuration

### Enable Auto-Withdrawal

Add to your `.env` file:

```env
# Enable automatic profit withdrawal
ENABLE_AUTO_WITHDRAW=true

# How often to check and withdraw (in seconds)
# Default: 3600 (1 hour)
WITHDRAW_INTERVAL=3600

# Minimum amount to withdraw (in wei, 0 = withdraw any amount)
# Default: 0 (withdraw any amount)
MIN_WITHDRAW_AMOUNT=0

# Token addresses to check for withdrawal (comma-separated)
# If empty, uses DEFAULT_COLLATERAL_ASSET
# Example: WITHDRAW_TOKENS=0x...,0x...,0x...
WITHDRAW_TOKENS=
```

### Example Configuration

```env
# Auto-withdraw profits every hour
ENABLE_AUTO_WITHDRAW=true
WITHDRAW_INTERVAL=3600  # 1 hour

# Only withdraw if balance > 0.1 tokens
MIN_WITHDRAW_AMOUNT=100000000000000000  # 0.1 token (18 decimals)

# Check specific tokens (optional)
WITHDRAW_TOKENS=0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14  # WETH aToken
```

---

## 🔄 Automatic Withdrawal

### How It Works

1. Bot runs in background
2. Every `WITHDRAW_INTERVAL` seconds, checks contract balance
3. If balance > `MIN_WITHDRAW_AMOUNT`, withdraws to your wallet
4. Logs withdrawal transactions

### Example Output

```
========================================
PHASE 4: Starting automatic profit withdrawal
========================================
Checking contract balances for withdrawal...
  0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14: Balance 1000000000000000000 - Withdrawing...
    ✓ Withdrawal tx sent: 0xabcd...
    Waiting for confirmation...
    ✓ Withdrawal confirmed in block 12345
========================================
PROFITS WITHDRAWN! 💰
Total withdrawn: 1000000000000000000
========================================
```

---

## 🖐️ Manual Withdrawal

### Option 1: Using Withdrawal Script

```bash
cd aave-bot

# Withdraw specific token
TOKEN_ADDRESS=0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14 \
go run scripts/withdrawProfits.go
```

### Option 2: Using FlashLoan Demo Scripts

```bash
cd ../flashloan-demo

# Withdraw aToken (redeems to underlying)
ATOKEN_ADDRESS=0x... \
FLASHLOAN_CONTRACT_ADDRESS=0x... \
npx hardhat run scripts/withdrawProfit.js --network sepolia

# Or withdraw any token
TOKEN_ADDRESS=0x... \
FLASHLOAN_CONTRACT_ADDRESS=0x... \
npx hardhat run scripts/withdrawProfit.js --network sepolia
```

---

## 🔍 Check Contract Balance

### From Bot

The bot automatically checks balances when withdrawing.

### From Script

```bash
cd flashloan-demo

# Check balance
TOKEN_ADDRESS=0x... \
FLASHLOAN_CONTRACT_ADDRESS=0x... \
npx hardhat run scripts/checkBalance.js --network sepolia
```

### From Go

```go
balance, err := flashLoan.GetContractBalance(ctx, tokenAddress)
```

---

## ⚠️ Important Notes

### 1. Contract Ownership
- **Only the contract owner can withdraw**
- Your bot wallet must be the contract owner
- Check ownership: The bot verifies this automatically

### 2. Withdrawal Types
- **aToken withdrawal** - Redeems aToken to underlying asset, then withdraws
- **Regular withdrawal** - Withdraws token directly (if not aToken)

### 3. Gas Costs
- Each withdrawal costs gas
- Set `MIN_WITHDRAW_AMOUNT` to avoid withdrawing tiny amounts
- Consider gas costs vs. withdrawal amount

### 4. Token Addresses
- Use **aToken addresses** for collateral (e.g., aWETH)
- Bot will try aToken withdrawal first, then regular withdrawal

---

## 📊 Configuration Examples

### Conservative (Withdraw Large Amounts Only)

```env
ENABLE_AUTO_WITHDRAW=true
WITHDRAW_INTERVAL=7200  # 2 hours
MIN_WITHDRAW_AMOUNT=1000000000000000000  # 1 token minimum
```

### Aggressive (Withdraw Frequently)

```env
ENABLE_AUTO_WITHDRAW=true
WITHDRAW_INTERVAL=1800  # 30 minutes
MIN_WITHDRAW_AMOUNT=0  # Withdraw any amount
```

### Manual Only (No Auto-Withdraw)

```env
ENABLE_AUTO_WITHDRAW=false
# Withdraw manually using scripts when needed
```

---

## 🎯 Finding aToken Addresses

To find the aToken address for a collateral asset:

```bash
# Using Aave Pool
# Call: pool.getReserveData(collateralAsset)
# Returns: reserveData.aTokenAddress
```

Or check Aave documentation for your network.

---

## ✅ Verification

After withdrawal, verify in your wallet:
- Check token balance increased
- Check transaction on explorer
- Verify amount matches contract balance

---

## 🆘 Troubleshooting

### "Not contract owner"
- Your wallet must be the contract owner
- Deploy contract with your wallet, or transfer ownership

### "No balance to withdraw"
- Contract has no profits yet
- Wait for successful liquidations
- Check balance manually

### "Withdrawal failed"
- Check gas price and limit
- Verify token address is correct
- Check contract has sufficient gas

---

## 📝 Summary

**Automatic:**
- Set `ENABLE_AUTO_WITHDRAW=true` in `.env`
- Bot withdraws profits periodically
- Configurable interval and minimum amount

**Manual:**
- Use `scripts/withdrawProfits.go`
- Or use flashloan-demo scripts
- Withdraw anytime you want

**Your profits are automatically sent to your wallet!** 💰
