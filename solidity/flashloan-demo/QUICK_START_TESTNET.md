# Quick Start: Sepolia Testnet (5 Minutes)

## 🚀 Fast Setup

### Step 1: Get Testnet ETH (1 min)
```bash
# Visit faucets:
# - https://sepoliafaucet.com/
# - https://www.infura.io/faucet/sepolia
# Get at least 0.1 Sepolia ETH
```

### Step 2: Configure (1 min)
```bash
cd flashloan-demo

# Edit .env
PRIVATE_KEY=your_testnet_key
SEPOLIA_RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_KEY
# Or: https://rpc.sepolia.org
```

### Step 3: Deploy Contract (1 min)
```bash
npx hardhat run scripts/deployProduction.js --network sepolia
# Save contract address: 0x...
```

### Step 4: Configure Bot (1 min)
```bash
cd ../aave-bot

# Edit .env
NETWORK=sepolia
RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_KEY
POOL_ADDRESS=0x6Ae43d3271ff6888e7Fc43Fd7321a503ff738951
FLASHLOAN_CONTRACT_ADDRESS=0x...  # From step 3
USE_FLASHLOAN_LIQUIDATION=true
ENABLE_AUTO_LIQUIDATION=false  # Monitor first
```

### Step 5: Run Bot (1 min)
```bash
go run main.go
```

---

## ✅ Done!

Your bot is now running on Sepolia testnet!

---

## 📝 Next Steps

- Monitor for liquidatable positions
- Test with small amounts
- Enable auto-liquidation when ready
- See `TESTNET_DEPLOYMENT.md` for details

---

## 🔗 Quick Links

- **Sepolia Explorer:** https://sepolia.etherscan.io
- **Aave Testnet:** https://staging.aave.com/
- **Faucet:** https://sepoliafaucet.com/

---

**Questions? See `TESTNET_DEPLOYMENT.md` for full guide.**
