# Quick Start: Profit Withdrawal

## ✅ Yes, You Can Withdraw Profits!

Your bot can automatically withdraw profits from the flash loan contract to your wallet.

---

## 🚀 Enable Auto-Withdrawal (Easiest)

Add to `aave-bot/.env`:

```env
# Enable automatic withdrawal
ENABLE_AUTO_WITHDRAW=true

# Withdraw every hour (3600 seconds)
WITHDRAW_INTERVAL=3600

# Withdraw any amount (set to 0)
MIN_WITHDRAW_AMOUNT=0
```

**That's it!** The bot will automatically:
- Check contract balance every hour
- Withdraw profits to your wallet
- Log all withdrawals

---

## 🖐️ Manual Withdrawal

### Option 1: Using Go Script

```bash
cd aave-bot

# Withdraw specific token
TOKEN_ADDRESS=0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14 \
go run scripts/withdrawProfits.go
```

### Option 2: Using FlashLoan Scripts

```bash
cd ../flashloan-demo

# Withdraw aToken (redeems to underlying)
ATOKEN_ADDRESS=0x... \
FLASHLOAN_CONTRACT_ADDRESS=0xcF78fAfC7D71A9899EB37aB308dC0537805c0b22 \
npx hardhat run scripts/withdrawProfit.js --network sepolia
```

---

## 📋 Configuration Options

| Variable | Default | Description |
|----------|---------|-------------|
| `ENABLE_AUTO_WITHDRAW` | `false` | Enable automatic withdrawal |
| `WITHDRAW_INTERVAL` | `3600` | Seconds between checks (1 hour) |
| `MIN_WITHDRAW_AMOUNT` | `0` | Minimum amount to withdraw (wei) |
| `WITHDRAW_TOKENS` | (empty) | Comma-separated token addresses |

---

## 💡 Recommended Settings

### For Testing (Frequent Withdrawals)
```env
ENABLE_AUTO_WITHDRAW=true
WITHDRAW_INTERVAL=1800  # 30 minutes
MIN_WITHDRAW_AMOUNT=0   # Withdraw any amount
```

### For Production (Efficient)
```env
ENABLE_AUTO_WITHDRAW=true
WITHDRAW_INTERVAL=3600  # 1 hour
MIN_WITHDRAW_AMOUNT=100000000000000000  # 0.1 token minimum
```

---

## ✅ Verification

After withdrawal:
1. Check your wallet balance increased
2. Check transaction on explorer
3. Bot logs will show withdrawal confirmation

---

**Your profits are automatically sent to your wallet!** 💰

See `PROFIT_WITHDRAWAL.md` for full documentation.
