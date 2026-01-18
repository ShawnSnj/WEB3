# Production Deployment Guide

This guide walks you through deploying and using the FlashLoan Liquidation system on real EVM networks.

---

## 🎯 Overview

The production system consists of:
1. **FlashLoanLiquidation Contract** - Smart contract with DEX integration
2. **Deployment Scripts** - Automated deployment to multiple networks
3. **Liquidation Scripts** - Execute liquidations on real networks
4. **Aave Bot Integration** - Monitor and liquidate automatically

---

## 📋 Prerequisites

### 1. Install Dependencies

```bash
cd flashloan-demo
npm install
```

### 2. Set Up Environment Variables

Create a `.env` file in the project root:

```bash
# Private key (NEVER commit this!)
PRIVATE_KEY=your_private_key_here

# RPC URLs (use your own API keys for better rate limits)
MAINNET_RPC_URL=https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY
ARBITRUM_RPC_URL=https://arb-mainnet.g.alchemy.com/v2/YOUR_API_KEY
POLYGON_RPC_URL=https://polygon-mainnet.g.alchemy.com/v2/YOUR_API_KEY
OPTIMISM_RPC_URL=https://opt-mainnet.g.alchemy.com/v2/YOUR_API_KEY
BASE_RPC_URL=https://mainnet.base.org

# Explorer API Keys (for contract verification)
ETHERSCAN_API_KEY=your_etherscan_api_key
ARBISCAN_API_KEY=your_arbiscan_api_key
POLYGONSCAN_API_KEY=your_polygonscan_api_key
OPTIMISTIC_ETHERSCAN_API_KEY=your_optimistic_etherscan_api_key
BASESCAN_API_KEY=your_basescan_api_key
```

**⚠️ Security:**
- Never commit `.env` file
- Use a dedicated wallet with limited funds
- Consider using a hardware wallet for production

---

## 🚀 Step 1: Deploy Contract

### Choose Your Network

Supported networks:
- `mainnet` - Ethereum Mainnet
- `arbitrum` - Arbitrum One
- `polygon` - Polygon
- `optimism` - Optimism
- `base` - Base

### Deploy

```bash
# Deploy to Ethereum Mainnet
npx hardhat run scripts/deployProduction.js --network mainnet

# Deploy to Arbitrum
npx hardhat run scripts/deployProduction.js --network arbitrum

# Deploy to Polygon
npx hardhat run scripts/deployProduction.js --network polygon
```

### What Happens

1. Contract is compiled with optimizations
2. Contract is deployed to the network
3. Configuration is verified
4. Deployment info is saved to `deployments/{network}.json`

### Example Output

```
=== Production Deployment ===

Network: arbitrum
Chain: Arbitrum One (Chain ID: 42161)
Explorer: https://arbiscan.io

Deployer: 0x1234...abcd
Balance: 0.5 ETH

=== Deployment Configuration ===
Aave PoolAddressesProvider: 0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb
Swap Router: 0xE592427A0AEce92De3Edee1F18E0157C05861564
Using Uniswap V3: true
Default Pool Fee: 3000

Deploying FlashLoanLiquidation contract...
Transaction sent, waiting for confirmation...

✓ Contract deployed!
Address: 0xABCD...EFGH
Explorer: https://arbiscan.io/address/0xABCD...EFGH
```

### Verify Contract (Optional)

```bash
npx hardhat verify --network arbitrum \
  <CONTRACT_ADDRESS> \
  <ADDRESS_PROVIDER> \
  <SWAP_ROUTER> \
  <USE_V3> \
  <POOL_FEE> \
  <SLIPPAGE_BPS>
```

---

## 💰 Step 2: Fund Contract (Optional)

The contract **doesn't require pre-funding** for normal operations (flash loans cover everything). However, funding can help with:
- Edge cases where swaps fail
- Gas optimization
- Emergency scenarios

### Fund with Native Token (ETH/ARB/etc.)

```bash
# Send ETH to contract (for gas refunds if needed)
# Use your wallet or:
npx hardhat run scripts/fundContract.js --network arbitrum
```

### Fund with Debt Tokens (Optional)

Only needed if you want to handle edge cases manually:

```bash
# Send USDC to contract
# Use your wallet to send tokens to the contract address
```

---

## ⚡ Step 3: Execute Liquidation

### Manual Liquidation

```bash
# Basic liquidation
VICTIM_ADDRESS=0x... \
npx hardhat run scripts/liquidateProduction.js --network arbitrum

# Custom parameters
VICTIM_ADDRESS=0x... \
DEBT_TOKEN=0xaf88d065e77c8cC2239327C5EDb3A432268e5831 \
COLLATERAL_TOKEN=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1 \
DEBT_AMOUNT=5000000000 \
FLASHLOAN_CONTRACT_ADDRESS=0x... \
npx hardhat run scripts/liquidateProduction.js --network arbitrum
```

### Parameters

- `VICTIM_ADDRESS` - Address with unhealthy position (required)
- `DEBT_TOKEN` - Token address to borrow (default: USDC)
- `COLLATERAL_TOKEN` - Collateral token address (default: WETH)
- `DEBT_AMOUNT` - Amount to liquidate (in token's smallest unit, e.g., 5000000000 = 5000 USDC)
- `FLASHLOAN_CONTRACT_ADDRESS` - Contract address (auto-loaded from deployments if not set)

---

## 🤖 Step 4: Integrate with Aave Bot

### Update Aave Bot Configuration

Edit `aave-bot/.env`:

```env
# Flash Loan Contract Address
FLASHLOAN_CONTRACT_ADDRESS=0xABCD...EFGH

# Enable flash loan liquidations
USE_FLASHLOAN_LIQUIDATION=true

# Network (must match contract deployment)
NETWORK=arbitrum
```

### Update Bot Code

The bot can call the flash loan contract instead of direct liquidations. See `aave-bot/integration/flashloan.go` for integration code.

---

## 📊 Step 5: Monitor & Withdraw Profits

### Check Contract Balance

```bash
npx hardhat run scripts/checkBalance.js --network arbitrum
```

### Withdraw Profits

```bash
# Withdraw aToken (collateral received from liquidation)
ATOKEN_ADDRESS=0x... \
npx hardhat run scripts/withdrawProfit.js --network arbitrum

# Or withdraw any token
TOKEN_ADDRESS=0x... \
npx hardhat run scripts/withdrawProfit.js --network arbitrum
```

---

## 🔒 Security Best Practices

### 1. Wallet Security
- ✅ Use a dedicated wallet for the bot
- ✅ Keep only necessary funds in the wallet
- ✅ Use hardware wallet for large operations
- ✅ Never share your private key

### 2. Contract Security
- ✅ Verify contract on explorer
- ✅ Review contract code before deployment
- ✅ Test on testnet first
- ✅ Start with small liquidations

### 3. Operational Security
- ✅ Monitor contract for unexpected activity
- ✅ Set up alerts for large transactions
- ✅ Regularly withdraw profits
- ✅ Keep private keys secure

### 4. Risk Management
- ✅ Set conservative slippage limits
- ✅ Monitor gas prices
- ✅ Test liquidation profitability before execution
- ✅ Have emergency withdrawal procedures

---

## 🧪 Testing Before Production

### 1. Test on Testnet

```bash
# Deploy to Sepolia testnet (if configured)
npx hardhat run scripts/deployProduction.js --network sepolia

# Test liquidation
VICTIM_ADDRESS=0x... \
npx hardhat run scripts/liquidateProduction.js --network sepolia
```

### 2. Test with Small Amounts

Start with small liquidations to verify everything works:

```bash
# Small liquidation (e.g., $100)
DEBT_AMOUNT=100000000 \
VICTIM_ADDRESS=0x... \
npx hardhat run scripts/liquidateProduction.js --network arbitrum
```

### 3. Monitor First Few Transactions

- Check transaction on explorer
- Verify profits are captured
- Ensure withdrawals work
- Check gas usage

---

## 📈 Network-Specific Notes

### Ethereum Mainnet
- **Gas Costs**: High (~$50-200 per transaction)
- **Best For**: Large liquidations (>$10k)
- **RPC**: Use premium RPC (Alchemy/Infura) for reliability

### Arbitrum
- **Gas Costs**: Very low (~$0.10-1 per transaction)
- **Best For**: Frequent small liquidations
- **RPC**: Public RPC usually sufficient

### Polygon
- **Gas Costs**: Very low (~$0.01-0.10)
- **Best For**: High-frequency operations
- **Note**: Lower liquidity than Ethereum/Arbitrum

### Optimism
- **Gas Costs**: Low (~$0.50-2)
- **Best For**: Medium-sized liquidations
- **RPC**: Use reliable RPC provider

### Base
- **Gas Costs**: Very low (~$0.10-0.50)
- **Best For**: New opportunities
- **Note**: Growing ecosystem

---

## 🐛 Troubleshooting

### "Insufficient funds for gas"
- Ensure wallet has native token (ETH/ARB/etc.) for gas
- Check gas price settings

### "Contract not found"
- Verify contract address is correct
- Check network matches deployment network
- Ensure contract was deployed successfully

### "Position not liquidatable"
- Health factor may have recovered
- Position may have been liquidated by someone else
- Verify victim address is correct

### "Swap failed"
- Check slippage settings
- Verify token addresses are correct
- Ensure sufficient liquidity in DEX pools

### "Transaction reverted"
- Check contract has proper permissions
- Verify all parameters are correct
- Review transaction on explorer for details

---

## 📞 Support

For issues:
1. Check transaction on explorer
2. Review contract events
3. Check network status
4. Verify configuration

---

## ⚠️ Disclaimer

**This software interacts with real financial protocols and uses real funds. Use at your own risk.**

- Always test thoroughly before production
- Start with small amounts
- Monitor operations closely
- The authors are not responsible for any losses
- Flash loan liquidations are competitive - others may liquidate first

---

## 🎉 Next Steps

1. ✅ Deploy contract to your chosen network
2. ✅ Test with a small liquidation
3. ✅ Set up monitoring
4. ✅ Integrate with your bot
5. ✅ Scale up gradually

Good luck with your liquidations! 🚀
