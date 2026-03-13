# Arbitrum Quick Start Guide

Fast deployment guide for both projects to Arbitrum.

---

## ⚡ Quick Deploy: uniswapFlashloan

```bash
# 1. Navigate to project
cd /Users/shawn/projects/WEB3/solidity/uniswapFlashloan

# 2. Install dependencies
npm install

# 3. Create .env file
cat > .env << EOF
PRIVATE_KEY=your_private_key_without_0x
ARBITRUM_RPC_URL=https://arb-mainnet.g.alchemy.com/v2/YOUR_API_KEY
ARBISCAN_API_KEY=your_arbiscan_key_optional
EOF

# 4. Compile
npm run compile

# 5. Deploy
npm run deploy:arbitrum
# or
npx hardhat run scripts/deploy.js --network arbitrum
```

**Save the contract address from output!**

---

## ⚡ Quick Setup: uniswap-bot

```bash
# 1. Navigate to project
cd /Users/shawn/projects/WEB3/solidity/uniswap-bot

# 2. Install dependencies
go mod download

# 3. Create .env file
cat > .env << EOF
PRIVATE_KEY=your_private_key_without_0x
RPC_URL=https://arb-mainnet.g.alchemy.com/v2/YOUR_API_KEY
UNISWAP_V2_ROUTER=0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24
FLASHLOAN_CONTRACT=0xYourDeployedContractAddressFromAbove
MONITOR_PAIRS=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1,0xaf88d065e77c8cC2239327C5EDb3A432268e5831
ENABLE_AUTO_SWAP=false
EOF

# 4. Build
go build -o uniswap-bot .

# 5. Run
./uniswap-bot
```

---

## 📋 Arbitrum Addresses Reference

### Core Contracts
```env
# Aave Pool Addresses Provider
AAVE_PROVIDER=0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb

# Uniswap V2 Router
UNISWAP_V2_ROUTER=0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24

# Uniswap V3 Router
UNISWAP_V3_ROUTER=0xE592427A0AEce92De3Edee1F18E0157C05861564
```

### Popular Tokens
```env
# WETH
WETH=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1

# USDC (Native)
USDC=0xaf88d065e77c8cC2239327C5EDb3A432268e5831

# USDT
USDT=0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9

# DAI
DAI=0xDA10009cBd5D07dd0CeCc66161FC93D7c9000da1
```

### Example Monitor Pairs
```env
# WETH/USDC
MONITOR_PAIRS=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1,0xaf88d065e77c8cC2239327C5EDb3A432268e5831

# WETH/USDT
MONITOR_PAIRS=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1,0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9
```

---

## 🔍 Verify Deployment

### Check Contract on Arbiscan
```
https://arbiscan.io/address/YOUR_CONTRACT_ADDRESS
```

### Check Deployment File
```bash
cat uniswapFlashloan/deployments/arbitrum.json
```

### Test Bot Connection
```bash
cd uniswap-bot
./uniswap-bot
# Should see: "Connected to Ethereum node"
```

---

## ⚠️ Important Notes

1. **Gas**: Arbitrum uses ETH (same as Ethereum) but much cheaper
2. **RPC**: Get free RPC from Alchemy/Infura (better than public)
3. **Private Key**: NEVER commit or share!
4. **Testing**: Start with `ENABLE_AUTO_SWAP=false` to monitor only

---

## 🆘 Troubleshooting

**"Failed to connect"**
→ Check RPC_URL is correct for Arbitrum

**"Insufficient funds"**
→ Fund wallet with ETH (0.1+ recommended)

**"Invalid address"**
→ Use Arbitrum addresses, not Ethereum mainnet!

**"Contract not found"**
→ Verify deployment succeeded and address is correct

---

For detailed instructions, see: `ARBITRUM_DEPLOYMENT_GUIDE.md`
