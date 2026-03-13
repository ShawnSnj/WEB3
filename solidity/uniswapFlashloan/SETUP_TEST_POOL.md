# Setting Up Test Pool on Sepolia

This guide helps you create a test Uniswap pool on Sepolia for testing your bot.

## Option 1: Use Existing Test Tokens

If test tokens already exist on Sepolia:

```bash
# Use existing Sepolia WETH
TOKEN0_ADDRESS=0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14 \
TOKEN1_ADDRESS=0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8 \
npx hardhat run scripts/setupTestPool.js --network sepolia
```

## Option 2: Deploy Your Own Test Tokens

### Step 1: Deploy Test Tokens

```bash
npx hardhat run scripts/deployTestTokens.js --network sepolia
```

This will deploy:
- TestUSDC (tUSDC) - 6 decimals, 1M supply
- TestWETH (tWETH) - 18 decimals, 1000 supply

**Save the deployed addresses!**

### Step 2: Create Pool

```bash
TOKEN0_ADDRESS=<testWETH_address> \
TOKEN1_ADDRESS=<testUSDC_address> \
npx hardhat run scripts/setupTestPool.js --network sepolia
```

### Step 3: Update Bot Configuration

```env
# Use your test token addresses
MONITOR_PAIRS=<testWETH_address>,<testUSDC_address>
```

## Option 3: Use Mainnet (Recommended for Testing)

Mainnet has many existing pools with liquidity:

```env
# Switch to mainnet
RPC_URL=https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY
UNISWAP_V2_ROUTER=0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D
MONITOR_PAIRS=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2,0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48
```

**Note:** Use small amounts and monitor gas costs!

## Prerequisites

1. **Sepolia ETH** for gas:
   - Get from: https://sepoliafaucet.com/
   - Or: https://www.alchemy.com/faucets/ethereum-sepolia

2. **Test Tokens** (if deploying):
   - Script will mint tokens to your address
   - Or get from test token faucets if available

## Troubleshooting

### "Pool already exists"
- Good! You can use the existing pool
- Check the pair address on Etherscan

### "Insufficient token balance"
- Get test tokens first
- Or deploy your own test tokens
- Or use existing tokens with balances

### "Factory address incorrect"
- Verify Uniswap V2 Factory address for Sepolia
- Check: https://docs.uniswap.org/contracts/v2/reference/smart-contracts/factory

## Quick Reference

**Sepolia Uniswap V2:**
- Factory: `0x7E0987E5b3a30e3f2828572Bb659A548460a3003`
- Router: `0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008`
- WETH: `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14`

**After Setup:**
1. Update bot's `MONITOR_PAIRS` with your token addresses
2. Restart bot
3. Bot should now detect prices!
