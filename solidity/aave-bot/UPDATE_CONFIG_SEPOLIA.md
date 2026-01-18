# Update Bot Config for Sepolia

## 📍 Config File Location

**Path:** `/Users/shawn/projects/WEB3/solidity/aave-bot/.env`

## 🔧 Update for Sepolia

Edit the `.env` file and update these values:

```env
# Network Configuration
NETWORK=sepolia
RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_API_KEY
# Or use public: https://rpc.sepolia.org

# Aave V3 Pool on Sepolia
POOL_ADDRESS=0x6Ae43d3271ff6888e7Fc43Fd7321a503ff738951

# Flash Loan Contract (from deployment)
USE_FLASHLOAN_LIQUIDATION=true
FLASHLOAN_CONTRACT_ADDRESS=0xcF78fAfC7D71A9899EB37aB308dC0537805c0b22

# Monitoring Settings
POLL_INTERVAL=5
HISTORICAL_BLOCKS_LOOKBACK=1000

# Liquidation Settings
ENABLE_AUTO_LIQUIDATION=false  # Start with monitoring only
LIQUIDATION_PROFIT_THRESHOLD=0.01
MAX_LIQUIDATION_AMOUNT=1000000000000000000  # 1 token

# Sepolia Testnet Tokens
DEFAULT_DEBT_ASSET=0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8  # USDC test
DEFAULT_COLLATERAL_ASSET=0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14  # WETH
```

## 📝 Quick Edit Command

```bash
cd /Users/shawn/projects/WEB3/solidity/aave-bot
nano .env
# or
code .env
# or
vim .env
```

## ✅ After Updating

1. Save the file
2. Run the bot: `go run main.go`
3. Check it connects to Sepolia
4. Monitor for liquidatable positions

---

**Config file:** `aave-bot/.env`
