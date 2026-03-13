# Troubleshooting Guide

## Common Issues

### "Price: 0.000000" or "No liquidity pool found"

**Problem:** The Uniswap pool doesn't exist or has no liquidity for your token pair.

**Causes:**
1. Using mainnet token addresses on testnet (or vice versa)
2. Pool doesn't exist on the network
3. Pool exists but has zero liquidity
4. Router address is incorrect for the network

**Solutions:**

#### 1. Verify Token Addresses

**Sepolia Testnet:**
```env
# Correct Sepolia addresses
MONITOR_PAIRS=0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14,0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8
# WETH, USDC
```

**Mainnet:**
```env
# Mainnet addresses
MONITOR_PAIRS=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2,0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48
# WETH, USDC
```

#### 2. Check Router Address

**Sepolia:**
```env
UNISWAP_V2_ROUTER=0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008
```

**Mainnet:**
```env
UNISWAP_V2_ROUTER=0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D
```

#### 3. Verify Pool Exists

Check on Etherscan/Uniswap:
- Visit Uniswap interface for your network
- Try swapping the tokens manually
- If swap fails, pool doesn't exist

#### 4. Create Test Pool (Sepolia)

On Sepolia, you may need to:
1. Deploy test tokens
2. Create a Uniswap pool
3. Add initial liquidity

**Quick Setup:**

```bash
cd ../uniswapFlashloan

# 1. Deploy test tokens
npx hardhat run scripts/deployTestTokens.js --network sepolia

# 2. Create pool and add liquidity
TOKEN0_ADDRESS=0x... TOKEN1_ADDRESS=0x... \
npx hardhat run scripts/setupTestPool.js --network sepolia
```

Or use existing test pools if available.

### "Failed to connect to Ethereum node"

**Solution:**
- Check `RPC_URL` is correct
- Verify API key is valid
- Try a different RPC provider
- Check network connectivity

### "Insufficient balance"

**Solution:**
- Ensure wallet has tokens to swap
- Check token addresses are correct
- Verify you have ETH for gas

### "Swap transaction reverted"

**Causes:**
- Price changed (slippage)
- Insufficient liquidity
- Pool doesn't exist
- Token approval failed

**Solution:**
- Increase `MAX_SLIPPAGE_BPS`
- Check pool has enough liquidity
- Verify token approval succeeded

## Network-Specific Issues

### Sepolia Testnet

**Common Problems:**
- Very few Uniswap pools exist
- Test tokens may not have pools
- Need to create pools manually

**Workaround:**
- Use mainnet for testing (with small amounts)
- Or deploy your own test tokens and create pools
- Check if Uniswap has deployed pools for your tokens

### Mainnet

**Common Problems:**
- High gas costs
- Slippage on volatile pairs
- Front-running

**Solution:**
- Use appropriate gas price multipliers
- Set conservative slippage
- Monitor gas costs vs. profits

## Debugging Tips

### 1. Enable Verbose Logging

Check logs for:
- Token addresses being used
- Router address
- Error messages

### 2. Test Router Connection

```bash
# Test if router is accessible
# Check on Etherscan: https://sepolia.etherscan.io/address/ROUTER_ADDRESS
```

### 3. Verify Token Contracts

```bash
# Check tokens exist
# Visit: https://sepolia.etherscan.io/address/TOKEN_ADDRESS
```

### 4. Check Pool on Uniswap

- Visit Uniswap interface
- Try to swap your tokens
- If it works, pool exists
- If it fails, pool doesn't exist

## Quick Fixes

### Switch to Mainnet (for testing)

```env
RPC_URL=https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY
UNISWAP_V2_ROUTER=0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D
MONITOR_PAIRS=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2,0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48
```

### Use Different Token Pair

Try a pair that definitely has a pool:
- WETH/USDC (mainnet)
- Check Uniswap for available pools on your network

## Still Having Issues?

1. **Check network match:** Token addresses, router, and RPC URL must all match the same network
2. **Verify pool exists:** Use Uniswap interface to confirm
3. **Test with small amounts:** Start with monitoring only
4. **Check logs:** Look for specific error messages
5. **Try mainnet:** Sepolia has very limited pools

See `SEPOLIA_SETUP.md` for Sepolia-specific configuration.
