# Flash Loan Integration Guide

This guide explains how to connect the **aave-bot** with the deployed **FlashLoanLiquidation** contract from the flashloan-demo project.

---

## 🔗 Overview

The aave-bot can now use flash loans for liquidations, which means:
- ✅ **No capital required** - Flash loans provide the funds
- ✅ **More opportunities** - Can liquidate larger positions
- ✅ **Lower risk** - No need to hold debt tokens

---

## 📋 Prerequisites

1. **Deploy FlashLoanLiquidation Contract**
   ```bash
   cd ../flashloan-demo
   npx hardhat run scripts/deployProduction.js --network arbitrum
   ```
   
   **Save the contract address!**

2. **Verify Deployment**
   - Check the contract on explorer
   - Verify it's the correct network
   - Note the contract address

---

## ⚙️ Configuration

### Update `.env` File

Add these variables to your `aave-bot/.env`:

```env
# Flash Loan Configuration
USE_FLASHLOAN_LIQUIDATION=true
FLASHLOAN_CONTRACT_ADDRESS=0xYourDeployedContractAddress

# Other settings remain the same
ENABLE_AUTO_LIQUIDATION=true
POOL_ADDRESS=0x794a61358D6845594F94dc1DB02A252b5b4814aD
DEFAULT_DEBT_ASSET=0xaf88d065e77c8cC2239327C5EDb3A432268e5831
DEFAULT_COLLATERAL_ASSET=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1
```

### Configuration Options

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `USE_FLASHLOAN_LIQUIDATION` | Yes | `false` | Set to `true` to enable flash loans |
| `FLASHLOAN_CONTRACT_ADDRESS` | Yes* | - | Contract address (required if flash loans enabled) |

*Required only if `USE_FLASHLOAN_LIQUIDATION=true`

---

## 🚀 Usage

### Option 1: Flash Loan Liquidations (Recommended)

```env
USE_FLASHLOAN_LIQUIDATION=true
FLASHLOAN_CONTRACT_ADDRESS=0x...
ENABLE_AUTO_LIQUIDATION=true
```

**Advantages:**
- No capital needed
- Can liquidate any size position
- Lower risk

**How it works:**
1. Bot detects liquidatable position
2. Calls flash loan contract
3. Contract borrows funds, liquidates, repays loan
4. Profit remains in contract
5. Owner withdraws profits later

### Option 2: Direct Liquidations (Original)

```env
USE_FLASHLOAN_LIQUIDATION=false
ENABLE_AUTO_LIQUIDATION=true
```

**Advantages:**
- Simpler (no contract dependency)
- Direct control

**Requirements:**
- Bot wallet must hold debt tokens
- Limited by wallet balance

---

## 🔄 Workflow

### Complete Integration Workflow

1. **Deploy Contract** (flashloan-demo)
   ```bash
   cd flashloan-demo
   npx hardhat run scripts/deployProduction.js --network arbitrum
   # Save contract address: 0xABCD...
   ```

2. **Update Bot Config** (aave-bot)
   ```bash
   cd aave-bot
   # Edit .env
   FLASHLOAN_CONTRACT_ADDRESS=0xABCD...
   USE_FLASHLOAN_LIQUIDATION=true
   ```

3. **Run Bot**
   ```bash
   go run main.go
   ```

4. **Monitor & Withdraw Profits**
   ```bash
   # Check contract balance
   cd flashloan-demo
   npx hardhat run scripts/checkBalance.js --network arbitrum
   
   # Withdraw profits
   npx hardhat run scripts/withdrawProfit.js --network arbitrum
   ```

---

## 📊 Comparison

| Feature | Direct Liquidation | Flash Loan Liquidation |
|---------|-------------------|----------------------|
| **Capital Required** | Yes (debt tokens) | No |
| **Max Position Size** | Limited by wallet | Unlimited |
| **Gas Cost** | Lower | Slightly higher |
| **Complexity** | Simple | Requires contract |
| **Risk** | Higher (hold tokens) | Lower (no tokens) |
| **Best For** | Small positions | Large positions |

---

## 🔧 Troubleshooting

### "FLASHLOAN_CONTRACT_ADDRESS not set"
- Set `FLASHLOAN_CONTRACT_ADDRESS` in `.env`
- Or set `USE_FLASHLOAN_LIQUIDATION=false` to disable

### "Failed to load flash loan contract"
- Verify contract address is correct
- Check network matches (Arbitrum vs Ethereum)
- Ensure contract is deployed

### "Transaction reverted"
- Check contract has proper permissions
- Verify victim position is still liquidatable
- Check gas limits

### "No profit detected"
- Profits go to contract, not bot wallet
- Withdraw from contract using flashloan-demo scripts
- Check contract balance

---

## 💡 Best Practices

1. **Start with Monitoring**
   ```env
   USE_FLASHLOAN_LIQUIDATION=true
   ENABLE_AUTO_LIQUIDATION=false  # Monitor first
   ```

2. **Test with Small Amounts**
   - Set low `MAX_LIQUIDATION_AMOUNT`
   - Test first few liquidations manually

3. **Monitor Contract**
   - Regularly check contract balance
   - Withdraw profits periodically
   - Monitor gas usage

4. **Network Selection**
   - **Arbitrum**: Lowest gas, recommended
   - **Ethereum**: Higher gas, more opportunities
   - **Polygon**: Very low gas, less liquidity

---

## 📝 Example Configuration

### Full Production Setup

```env
# Network
NETWORK=arbitrum
RPC_URL=https://arb1.arbitrum.io/rpc

# Aave
POOL_ADDRESS=0x794a61358D6845594F94dc1DB02A252b5b4814aD

# Flash Loan (from flashloan-demo deployment)
USE_FLASHLOAN_LIQUIDATION=true
FLASHLOAN_CONTRACT_ADDRESS=0xYourDeployedContractAddress

# Bot Settings
ENABLE_AUTO_LIQUIDATION=true
POLL_INTERVAL=5
HISTORICAL_BLOCKS_LOOKBACK=5000

# Liquidation Settings
MAX_LIQUIDATION_AMOUNT=1000000000000000000000  # 1000 tokens
LIQUIDATION_PROFIT_THRESHOLD=0.01  # 1%

# Tokens (Arbitrum)
DEFAULT_DEBT_ASSET=0xaf88d065e77c8cC2239327C5EDb3A432268e5831  # USDC
DEFAULT_COLLATERAL_ASSET=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1  # WETH
```

---

## 🔗 Related Documentation

- **FlashLoan Demo**: `../flashloan-demo/PRODUCTION_DEPLOYMENT.md`
- **Bot README**: `README.md`
- **Finding Victims**: `../flashloan-demo/FINDING_VICTIMS.md`

---

## ⚠️ Important Notes

1. **Contract Ownership**: Flash loan contract owner can withdraw profits
2. **Network Matching**: Contract and bot must be on same network
3. **Gas Costs**: Flash loans use slightly more gas
4. **Competition**: Others may liquidate first
5. **Testing**: Always test with small amounts first

---

## 🎉 Next Steps

1. ✅ Deploy flash loan contract
2. ✅ Update bot configuration
3. ✅ Test with monitoring mode
4. ✅ Enable auto-liquidation
5. ✅ Monitor and withdraw profits

**Ready to integrate? Follow the workflow above!** 🚀
