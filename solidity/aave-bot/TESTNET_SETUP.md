# Aave Bot - Sepolia Testnet Setup

Quick guide for running the bot on Sepolia testnet.

---

## ⚙️ Configuration

### Update `.env`:

```env
# Network
NETWORK=sepolia
RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_API_KEY
# Or: https://rpc.sepolia.org

# Aave V3 Sepolia
POOL_ADDRESS=0x6Ae43d3271ff6888e7Fc43Fd7321a503ff738951

# Flash Loan Contract (deploy first from flashloan-demo)
USE_FLASHLOAN_LIQUIDATION=true
FLASHLOAN_CONTRACT_ADDRESS=0xYourDeployedContractAddress

# Bot Settings
ENABLE_AUTO_LIQUIDATION=false  # Start with monitoring
POLL_INTERVAL=5
HISTORICAL_BLOCKS_LOOKBACK=1000

# Liquidation Settings
MAX_LIQUIDATION_AMOUNT=1000000000000000000  # 1 token
LIQUIDATION_PROFIT_THRESHOLD=0.01

# Testnet Tokens (Sepolia)
DEFAULT_DEBT_ASSET=0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8  # USDC test
DEFAULT_COLLATERAL_ASSET=0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14  # WETH

# Wallet
PRIVATE_KEY=your_testnet_private_key
```

---

## 🚀 Quick Start

1. **Get Sepolia ETH:**
   - https://sepoliafaucet.com/
   - https://www.infura.io/faucet/sepolia

2. **Deploy FlashLoan Contract:**
   ```bash
   cd ../flashloan-demo
   npx hardhat run scripts/deployProduction.js --network sepolia
   ```

3. **Update Bot Config:**
   - Add `FLASHLOAN_CONTRACT_ADDRESS` to `.env`

4. **Run Bot:**
   ```bash
   go run main.go
   ```

---

## 📊 Sepolia Addresses

- **Aave Pool:** `0x6Ae43d3271ff6888e7Fc43Fd7321a503ff738951`
- **WETH:** `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14`
- **USDC Test:** `0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8`

---

## ⚠️ Notes

- Testnet has low activity - fewer liquidatable positions
- May need to create test positions yourself
- Use Aave testnet UI: https://staging.aave.com/

---

See `../flashloan-demo/TESTNET_DEPLOYMENT.md` for full guide.
