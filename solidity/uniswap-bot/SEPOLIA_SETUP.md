# Sepolia Testnet Setup Guide

## Token Addresses for Sepolia

**Important:** Sepolia uses different token addresses than mainnet!

### Sepolia Token Addresses

```env
# Sepolia WETH
WETH_SEPOLIA=0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14

# Sepolia USDC (test token)
USDC_SEPOLIA=0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8

# Sepolia DAI (test token)
DAI_SEPOLIA=0x3e622317f8C93f7328350cF0B56d9eD4C620C5d6
```

### Configuration for Sepolia

Update your `.env` file:

```env
# Network
RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_API_KEY

# Uniswap Router (Sepolia)
UNISWAP_V2_ROUTER=0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008

# Token pairs (Sepolia addresses)
MONITOR_PAIRS=0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14,0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8
# Format: WETH,USDC

# Flash loan contract (if deployed)
USE_FLASHLOAN=true
FLASHLOAN_CONTRACT_ADDRESS=0x585931F74c8Be7Ba53A1f649E510Ad5C34D78D1f
```

## Common Issues

### "No liquidity pool found"

**Cause:** Using mainnet token addresses on Sepolia, or pool doesn't exist.

**Solution:**
1. Use Sepolia token addresses (see above)
2. Check if a Uniswap pool exists for your token pair
3. You may need to create a test pool first

### "Failed to get amounts out"

**Cause:** 
- Token addresses are wrong for the network
- Pool doesn't exist
- Router address is incorrect

**Solution:**
1. Verify token addresses match the network
2. Check router address is correct for Sepolia
3. Ensure a Uniswap pool exists for the pair

## Getting Test Tokens

### Sepolia ETH
- https://sepoliafaucet.com/
- https://faucet.quicknode.com/ethereum/sepolia
- https://www.alchemy.com/faucets/ethereum-sepolia

### Test Tokens
- Some test tokens may be available on Sepolia
- You may need to deploy your own test tokens
- Or use existing test tokens from Uniswap

## Testing

1. **Verify token addresses:**
   ```bash
   # Check if tokens exist on Sepolia
   # Visit: https://sepolia.etherscan.io/address/TOKEN_ADDRESS
   ```

2. **Check if pool exists:**
   - Uniswap pools may not exist for all pairs on Sepolia
   - You may need to create a test pool first

3. **Start with monitoring:**
   ```env
   ENABLE_AUTO_SWAP=false  # Monitor only first
   ```

## Mainnet vs Sepolia

| Token | Mainnet | Sepolia |
|-------|---------|---------|
| WETH | `0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2` | `0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14` |
| USDC | `0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48` | `0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8` |
| Router | `0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D` | `0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008` |

**Always use network-specific addresses!**
