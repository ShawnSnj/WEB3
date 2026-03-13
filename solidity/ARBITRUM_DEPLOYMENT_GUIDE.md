# Arbitrum Deployment Guide

Complete guide to deploy **uniswap-bot** and **uniswapFlashloan** to Arbitrum One.

---

## 📋 Prerequisites

1. **Node.js & npm** (for uniswapFlashloan)
2. **Go 1.21+** (for uniswap-bot)
3. **Arbitrum RPC URL** (Alchemy, Infura, or public)
4. **Wallet with ARB** for gas fees (at least 0.1 ARB recommended)
5. **Private key** (NEVER share or commit!)

---

## 🌐 Arbitrum Network Information

- **Network Name**: Arbitrum One
- **Chain ID**: 42161
- **RPC URL**: `https://arb1.arbitrum.io/rpc` (public) or Alchemy/Infura
- **Block Explorer**: https://arbiscan.io/
- **Native Token**: ETH (same as Ethereum, but cheaper gas!)

---

## Part 1: Deploy uniswapFlashloan Contract

### Step 1: Navigate to Project

```bash
cd /Users/shawn/projects/WEB3/solidity/uniswapFlashloan
```

### Step 2: Install Dependencies

```bash
npm install
```

### Step 3: Create/Update `.env` File

Create a `.env` file in the project root:

```env
# Private key (NEVER commit this!)
PRIVATE_KEY=your_private_key_here_without_0x_prefix

# Arbitrum RPC URL
ARBITRUM_RPC_URL=https://arb-mainnet.g.alchemy.com/v2/YOUR_API_KEY
# Or use public: https://arb1.arbitrum.io/rpc

# Optional: For contract verification on Arbiscan
ARBISCAN_API_KEY=your_arbiscan_api_key

# Optional: Deployment parameters
USE_V3_ROUTER=false
DEFAULT_POOL_FEE=3000
MAX_SLIPPAGE_BPS=50
```

**Get RPC URL:**
- **Alchemy**: https://www.alchemy.com/ → Create app → Arbitrum → Copy HTTP URL
- **Infura**: https://www.infura.io/ → Create project → Arbitrum → Copy URL
- **Public**: `https://arb1.arbitrum.io/rpc` (rate-limited)

**Get Arbiscan API Key** (optional, for verification):
- Go to https://arbiscan.io/
- Sign up → API Keys → Create API Key

### Step 4: Compile Contracts

```bash
npm run compile
# or
npx hardhat compile
```

### Step 5: Deploy to Arbitrum

```bash
npx hardhat run scripts/deploy.js --network arbitrum
```

**Expected Output:**
```
Deploying to arbitrum...
Deploying with account: 0xYourAddress
Account balance: 0.5 ETH

Deployment parameters:
  Aave Pool Addresses Provider: 0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb
  Uniswap V2 Router: 0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24
  Uniswap V3 Router: 0xE592427A0AEce92De3Edee1F18E0157C05861564
  Use V3 Router: false
  Default Pool Fee: 3000
  Max Slippage (BPS): 50

Deploying contract...

========================================
DEPLOYMENT SUCCESSFUL!
========================================
Contract address: 0xYourDeployedContractAddress
Network: arbitrum
Deployer: 0xYourAddress
========================================

Deployment info saved to: deployments/arbitrum.json
```

### Step 6: Verify Contract (Optional)

The script will automatically verify if `ARBISCAN_API_KEY` is set in `.env`.

Or verify manually:
```bash
npx hardhat verify --network arbitrum \
  CONTRACT_ADDRESS \
  "0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb" \
  "0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24" \
  "0xE592427A0AEce92De3Edee1F18E0157C05861564" \
  false \
  3000 \
  50
```

### Step 7: Save Deployment Address

The deployment address is saved in `deployments/arbitrum.json`. **Save this address** - you'll need it for the Go bot!

---

## Part 2: Configure uniswap-bot (Go)

### Step 1: Navigate to Project

```bash
cd /Users/shawn/projects/WEB3/solidity/uniswap-bot
```

### Step 2: Install Dependencies

```bash
go mod download
```

### Step 3: Create/Update `.env` File

Create a `.env` file in the project root:

```env
# Private key (NEVER commit this!)
PRIVATE_KEY=your_private_key_here_without_0x_prefix

# Arbitrum RPC URL
RPC_URL=https://arb-mainnet.g.alchemy.com/v2/YOUR_API_KEY
# Or use public: https://arb1.arbitrum.io/rpc

# Uniswap V2 Router on Arbitrum
UNISWAP_V2_ROUTER=0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24

# Flash Loan Contract (from Part 1 deployment)
FLASHLOAN_CONTRACT=0xYourDeployedContractAddressFromPart1

# Token pairs to monitor (comma-separated)
# Format: TOKEN0_ADDRESS,TOKEN1_ADDRESS
# Arbitrum WETH/USDC example:
MONITOR_PAIRS=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1,0xaf88d065e77c8cC2239327C5EDb3A432268e5831

# Monitoring settings
POLL_INTERVAL=5
MIN_PROFIT_PERCENTAGE=0.01

# Swap settings
MAX_SLIPPAGE_BPS=50
ENABLE_AUTO_SWAP=false

# Flash loan settings (if integrated)
ENABLE_FLASHLOAN=false
FLASHLOAN_PROFIT_THRESHOLD=0.02
```

### Step 4: Arbitrum Token Addresses

**Popular Arbitrum Tokens:**

```env
# WETH (Wrapped ETH)
WETH=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1

# USDC (Native)
USDC=0xaf88d065e77c8cC2239327C5EDb3A432268e5831

# USDC.e (Bridged)
USDC_E=0xFF970A61A04b1cA14834A43f5dE4533eBDDB5CC8

# USDT
USDT=0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9

# DAI
DAI=0xDA10009cBd5D07dd0CeCc66161FC93D7c9000da1

# ARB Token
ARB=0x912CE59144191C1204E64559FE8253a0e49E6548
```

**Example Monitor Pairs:**
```env
# WETH/USDC
MONITOR_PAIRS=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1,0xaf88d065e77c8cC2239327C5EDb3A432268e5831

# WETH/USDT
MONITOR_PAIRS=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1,0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9

# Multiple pairs (comma-separated, pairs separated by commas)
MONITOR_PAIRS=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1,0xaf88d065e77c8cC2239327C5EDb3A432268e5831,0x82aF49447D8a07e3bd95BD0d56f35241523fBab1,0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9
```

### Step 5: Build the Bot

```bash
go build -o uniswap-bot .
```

### Step 6: Test Connection

```bash
# Run with monitoring only (no auto-swap)
./uniswap-bot
```

**Expected Output:**
```
Configuration loaded:
  Wallet: 0xYourAddress
  Router: 0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24
  RPC: https://arb-mainnet.g.alchemy.com/v2/...

Connected to Ethereum node

Starting price monitoring...
Monitoring 1 token pair(s)
Poll interval: 5s
Min profit: 1.00%
Auto-swap: false

[WETH/USDC] Price: 2500.000000 (1 token0 = 2500.000000 token1)
```

---

## 🔗 Integration: Connect Go Bot to Flash Loan Contract

If you want the Go bot to use the flash loan contract:

### Step 1: Update Go Bot Code

Check if `integration/flashloan.go` exists and update it with your deployed contract address.

### Step 2: Enable Flash Loan in `.env`

```env
ENABLE_FLASHLOAN=true
FLASHLOAN_CONTRACT=0xYourDeployedContractAddress
FLASHLOAN_PROFIT_THRESHOLD=0.02  # 2% minimum profit for flash loan
```

### Step 3: Fund Contract (if needed)

Some flash loan contracts need to be pre-funded for premiums. Check the contract requirements.

---

## ✅ Verification Checklist

After deployment, verify:

- [ ] Contract deployed successfully
- [ ] Contract address saved in `deployments/arbitrum.json`
- [ ] Contract verified on Arbiscan (optional but recommended)
- [ ] Go bot connects to Arbitrum RPC
- [ ] Go bot can read token prices
- [ ] Wallet has ARB/ETH for gas
- [ ] `.env` files are in `.gitignore` (NEVER commit private keys!)

---

## 🧪 Testing on Arbitrum

### Test 1: Price Monitoring

```bash
# Run bot with monitoring only
ENABLE_AUTO_SWAP=false ./uniswap-bot
```

Should see price updates every 5 seconds.

### Test 2: Manual Swap (Optional)

```bash
# Enable auto-swap (CAREFUL - uses real funds!)
ENABLE_AUTO_SWAP=true ./uniswap-bot
```

**⚠️ Warning**: Only do this with small amounts you're comfortable losing!

### Test 3: Flash Loan (if integrated)

```bash
# Enable flash loan arbitrage
ENABLE_FLASHLOAN=true ./uniswap-bot
```

---

## 🚨 Common Issues & Solutions

### Issue: "Failed to connect to Arbitrum node"

**Solution:**
- Check `RPC_URL` is correct
- Verify RPC provider is working
- Try a different RPC endpoint
- Check internet connection

### Issue: "Insufficient funds for gas"

**Solution:**
- Fund your wallet with ARB/ETH
- Arbitrum uses ETH for gas (but cheaper than mainnet)
- Minimum recommended: 0.1 ETH

### Issue: "Contract deployment failed"

**Solution:**
- Check you have enough ETH for gas
- Verify private key is correct
- Check RPC URL is for Arbitrum (chain ID 42161)
- Try increasing gas limit in Hardhat config

### Issue: "Token not found" or "Invalid address"

**Solution:**
- Verify token addresses are correct for Arbitrum
- Use addresses from official Arbitrum token list
- Check addresses on Arbiscan

### Issue: "No profitable opportunities"

**Solution:**
- This is normal - profitable opportunities are rare
- Lower `MIN_PROFIT_PERCENTAGE` (but be careful!)
- Check you have sufficient token balances
- Verify pools have liquidity

---

## 📊 Arbitrum-Specific Considerations

### Gas Costs

- **Much cheaper** than Ethereum mainnet
- Typical transaction: ~0.0001-0.001 ETH
- Flash loans: ~0.001-0.01 ETH

### Network Speed

- **Faster** than Ethereum mainnet
- Block time: ~0.25 seconds
- Confirmations: Usually 1-2 blocks

### Token Differences

- **WETH**: Same concept, different address
- **USDC**: Two versions (native and bridged)
- **Native tokens**: Use Arbitrum addresses, not Ethereum!

---

## 🔐 Security Best Practices

1. **Never commit `.env` files**
   ```bash
   # Make sure .env is in .gitignore
   echo ".env" >> .gitignore
   ```

2. **Use separate wallet for bot**
   - Don't use your main wallet
   - Only fund with amounts you're comfortable trading

3. **Start with monitoring only**
   ```env
   ENABLE_AUTO_SWAP=false
   ENABLE_FLASHLOAN=false
   ```

4. **Test with small amounts first**
   - Start with minimal funds
   - Monitor closely
   - Gradually increase if confident

5. **Keep private keys secure**
   - Never share or commit
   - Use environment variables
   - Consider hardware wallet for large amounts

---

## 📝 Quick Reference

### Deploy Flash Loan Contract
```bash
cd uniswapFlashloan
npm install
# Set .env with PRIVATE_KEY and ARBITRUM_RPC_URL
npx hardhat run scripts/deploy.js --network arbitrum
```

### Run Go Bot
```bash
cd uniswap-bot
go mod download
# Set .env with PRIVATE_KEY, RPC_URL, and token addresses
go build -o uniswap-bot .
./uniswap-bot
```

### Check Deployment
```bash
# View deployment info
cat uniswapFlashloan/deployments/arbitrum.json

# Check on Arbiscan
# https://arbiscan.io/address/YOUR_CONTRACT_ADDRESS
```

---

## 🎯 Next Steps

1. ✅ Deploy flash loan contract
2. ✅ Configure Go bot
3. ✅ Test price monitoring
4. ✅ Test manual swap (small amount)
5. ✅ Integrate flash loan (if needed)
6. ✅ Monitor and optimize

---

## 📚 Additional Resources

- **Arbitrum Docs**: https://docs.arbitrum.io/
- **Arbiscan**: https://arbiscan.io/
- **Aave Arbitrum**: https://docs.aave.com/developers/deployed-contracts/v3-mainnet/arbitrum
- **Uniswap Arbitrum**: https://docs.uniswap.org/contracts/v2/reference/smart-contracts/router-02

---

## ⚠️ Disclaimer

Deploying and using these bots involves financial risk. Always:
- Test thoroughly on testnets first
- Start with small amounts
- Monitor closely
- Understand the risks
- Never share your private keys

Good luck! 🚀
