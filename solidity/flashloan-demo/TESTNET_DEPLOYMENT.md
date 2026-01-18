# Testnet Deployment Guide (Sepolia)

This guide walks you through deploying and testing on **Sepolia testnet**.

---

## 🎯 Prerequisites

### 1. Get Testnet ETH

You need Sepolia ETH for gas fees. Get it from:
- **Faucets:**
  - [Alchemy Sepolia Faucet](https://sepoliafaucet.com/)
  - [Infura Sepolia Faucet](https://www.infura.io/faucet/sepolia)
  - [Chainlink Faucet](https://faucets.chain.link/sepolia)
  - [PoW Faucet](https://sepolia-faucet.pk910.de/)

**Recommended:** Get at least 0.1 Sepolia ETH

### 2. Get Testnet Tokens (Optional)

For testing liquidations, you may need test tokens:
- **USDC Test Token:** `0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8`
- **DAI Test Token:** `0x3e622317f8C93f7328350cF0B56d9eD4C620C5d6`
- **WETH:** `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14`

---

## 📋 Step 1: Configure Environment

### Update `.env` file:

```bash
# Private key (testnet wallet - use a separate wallet!)
PRIVATE_KEY=your_testnet_private_key_here

# Sepolia RPC (use your own API key for better rate limits)
SEPOLIA_RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_API_KEY
# Or use public: https://rpc.sepolia.org

# Explorer API Key (for contract verification)
ETHERSCAN_API_KEY=your_etherscan_api_key
```

**Get RPC URLs:**
- **Alchemy:** https://www.alchemy.com/ (free tier available)
- **Infura:** https://www.infura.io/ (free tier available)
- **Public:** `https://rpc.sepolia.org` (rate limited)

---

## 🚀 Step 2: Deploy FlashLoan Contract

### Deploy to Sepolia:

```bash
cd flashloan-demo

# Deploy contract
npx hardhat run scripts/deployProduction.js --network sepolia
```

### Expected Output:

```
=== Production Deployment ===

Network: sepolia
Chain: Sepolia Testnet (Chain ID: 11155111)
Explorer: https://sepolia.etherscan.io

Deployer: 0x...
Balance: 0.1 ETH

=== Deployment Configuration ===
Aave PoolAddressesProvider: 0x012bAC54348C0E635dCAc19D0f3C925d2C0Cb0e
Swap Router: 0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008
Using Uniswap V3: false
Default Pool Fee: 3000

Deploying FlashLoanLiquidation contract...
Transaction sent, waiting for confirmation...

✓ Contract deployed!
Address: 0xABCD...
Explorer: https://sepolia.etherscan.io/address/0xABCD...
```

**Save the contract address!**

### Verify Contract (Optional):

```bash
npx hardhat verify --network sepolia \
  <CONTRACT_ADDRESS> \
  <ADDRESS_PROVIDER> \
  <SWAP_ROUTER> \
  <USE_V3> \
  <POOL_FEE> \
  <SLIPPAGE_BPS>
```

---

## 🤖 Step 3: Configure Aave Bot

### Update `aave-bot/.env`:

```env
# Network
NETWORK=sepolia
RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_API_KEY

# Aave V3 Sepolia
POOL_ADDRESS=0x6Ae43d3271ff6888e7Fc43Fd7321a503ff738951

# Flash Loan Contract (from step 2)
USE_FLASHLOAN_LIQUIDATION=true
FLASHLOAN_CONTRACT_ADDRESS=0xYourDeployedContractAddress

# Bot Settings
ENABLE_AUTO_LIQUIDATION=false  # Start with monitoring only
POLL_INTERVAL=5
HISTORICAL_BLOCKS_LOOKBACK=1000

# Liquidation Settings
MAX_LIQUIDATION_AMOUNT=1000000000000000000  # 1 token (test amount)
LIQUIDATION_PROFIT_THRESHOLD=0.01  # 1%

# Testnet Tokens (Sepolia)
DEFAULT_DEBT_ASSET=0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8  # USDC test
DEFAULT_COLLATERAL_ASSET=0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14  # WETH

# Private Key (same testnet wallet)
PRIVATE_KEY=your_testnet_private_key_here
```

---

## 🧪 Step 4: Test the Bot

### Start Monitoring:

```bash
cd aave-bot

# Run bot in monitoring mode
go run main.go
```

### Expected Output:

```
Configuration loaded:
  Pool Address: 0x6Ae43d3271ff6888e7Fc43Fd7321a503ff738951
  Poll Interval: 5s
  Historical Lookback: 1000 blocks
  Auto-Liquidation: false
  Flash Loan Liquidation: true
  Flash Loan Contract: 0xYourContractAddress

PHASE 1: Discovering addresses from historical events
Scanning historical events from block X to Y
Found N unique addresses with Aave positions

PHASE 2: Starting health factor monitoring
Checking health factors for N addresses...
```

### Enable Auto-Liquidation (After Testing):

```env
ENABLE_AUTO_LIQUIDATION=true
```

---

## 💡 Step 5: Test Liquidation Manually

### Option 1: Using Script

```bash
cd flashloan-demo

# Find a liquidatable position first
node scripts/findVictimsSubgraph.js sepolia

# Execute liquidation
VICTIM_ADDRESS=0x... \
FLASHLOAN_CONTRACT_ADDRESS=0x... \
npx hardhat run scripts/liquidateProduction.js --network sepolia
```

### Option 2: Using Bot

1. Set `ENABLE_AUTO_LIQUIDATION=true` in bot `.env`
2. Bot will automatically liquidate when it finds opportunities

---

## 📊 Sepolia Testnet Details

### Network Info:
- **Chain ID:** 11155111
- **RPC:** `https://rpc.sepolia.org` (public) or use Alchemy/Infura
- **Explorer:** https://sepolia.etherscan.io
- **Block Time:** ~12 seconds

### Aave V3 Sepolia:
- **Pool:** `0x6Ae43d3271ff6888e7Fc43Fd7321a503ff738951`
- **PoolAddressesProvider:** `0x012bAC54348C0E635dCAc19D0f3C925d2C0Cb0e`

### Test Tokens:
- **WETH:** `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14`
- **USDC:** `0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8`
- **DAI:** `0x3e622317f8C93f7328350cF0B56d9eD4C620C5d6`

### Uniswap Sepolia:
- **V2 Router:** `0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008`
- **V3 Router:** May not be fully available (using V2)

---

## ⚠️ Important Notes

### 1. Testnet Limitations
- **Low Activity:** Fewer liquidatable positions than mainnet
- **Test Tokens:** May need to mint/get test tokens
- **Liquidity:** DEX pools may have low liquidity

### 2. Finding Test Positions
- Use Aave testnet UI: https://staging.aave.com/
- Create test positions yourself
- Monitor for positions going unhealthy

### 3. Gas Costs
- Sepolia ETH is free (from faucets)
- Gas prices are usually low
- Good for testing without cost

### 4. Contract Verification
- Verify on Sepolia Etherscan for transparency
- Helps with debugging
- Makes contract code visible

---

## 🔧 Troubleshooting

### "Insufficient funds for gas"
- Get more Sepolia ETH from faucets
- Check wallet balance

### "No liquidatable positions found"
- Normal on testnet (low activity)
- Create test positions yourself
- Use Aave testnet UI to borrow

### "Contract not found"
- Verify contract address is correct
- Check network matches (Sepolia)
- Ensure contract was deployed successfully

### "RPC rate limit exceeded"
- Use your own RPC endpoint (Alchemy/Infura)
- Add delays between requests
- Use public RPC as fallback

---

## 📝 Quick Test Checklist

- [ ] Got Sepolia ETH (0.1+)
- [ ] Deployed FlashLoan contract
- [ ] Saved contract address
- [ ] Updated bot `.env` with contract address
- [ ] Bot connects to Sepolia RPC
- [ ] Bot discovers addresses
- [ ] Bot monitors health factors
- [ ] Tested manual liquidation (optional)
- [ ] Verified contract on explorer

---

## 🎉 Next Steps

After successful testnet deployment:

1. **Test thoroughly** on Sepolia
2. **Monitor** for a few days
3. **Verify** all functionality works
4. **Deploy to mainnet** when ready (see `PRODUCTION_DEPLOYMENT.md`)

---

## 🔗 Useful Links

- **Sepolia Explorer:** https://sepolia.etherscan.io
- **Aave Testnet:** https://staging.aave.com/
- **Sepolia Faucet:** https://sepoliafaucet.com/
- **Alchemy:** https://www.alchemy.com/
- **Infura:** https://www.infura.io/

---

**Ready to test? Start with Step 1!** 🚀
