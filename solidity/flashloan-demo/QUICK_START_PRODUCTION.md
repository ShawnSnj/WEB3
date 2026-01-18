# Quick Start: Production Deployment

## 🚀 5-Minute Setup

### Step 1: Install & Configure (2 min)

```bash
cd flashloan-demo
npm install

# Create .env file
cat > .env << EOF
PRIVATE_KEY=your_private_key_here
ARBITRUM_RPC_URL=https://arb1.arbitrum.io/rpc
ARBISCAN_API_KEY=your_key_here
EOF
```

### Step 2: Deploy Contract (1 min)

```bash
# Deploy to Arbitrum (lowest gas costs)
npx hardhat run scripts/deployProduction.js --network arbitrum
```

**Save the contract address from output!**

### Step 3: Find a Victim (1 min)

```bash
# Find liquidatable positions
node scripts/findVictimsSubgraph.js arbitrum

# Or use event scanning
npx hardhat run scripts/findVictims.js --network arbitrum
```

### Step 4: Execute Liquidation (1 min)

```bash
# Use victim address from step 3
VICTIM_ADDRESS=0x... \
FLASHLOAN_CONTRACT_ADDRESS=0x... \
npx hardhat run scripts/liquidateProduction.js --network arbitrum
```

### Step 5: Withdraw Profits

```bash
# Withdraw aToken profits
ATOKEN_ADDRESS=0x... \
FLASHLOAN_CONTRACT_ADDRESS=0x... \
npx hardhat run scripts/withdrawProfit.js --network arbitrum
```

---

## ✅ That's It!

You now have:
- ✅ Production contract deployed
- ✅ First liquidation executed
- ✅ Profits withdrawn

---

## 📖 Next Steps

- Read `PRODUCTION_DEPLOYMENT.md` for detailed guide
- Integrate with `aave-bot` for automation
- Set up monitoring
- Scale up gradually

---

## ⚠️ Important

- **Test with small amounts first**
- **Use dedicated wallet**
- **Monitor gas prices**
- **Start on Arbitrum** (lowest costs)

---

**Questions? See `PRODUCTION_DEPLOYMENT.md` for full documentation.**
