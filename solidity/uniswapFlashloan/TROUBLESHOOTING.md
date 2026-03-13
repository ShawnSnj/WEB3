# Troubleshooting Guide

## Common Issues

### "Must be authenticated!" Error

**Problem:** RPC URL is missing or has an invalid API key.

**Solution:**

1. Create a `.env` file in the project root:
   ```bash
   cd uniswapFlashloan
   touch .env
   ```

2. Add your RPC URL with a valid API key:
   ```env
   # For Sepolia testnet
   SEPOLIA_RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_ACTUAL_API_KEY
   
   # Or use a public RPC (less reliable)
   SEPOLIA_RPC_URL=https://rpc.sepolia.org
   
   # Private key for deployment
   PRIVATE_KEY=your_private_key_here
   ```

3. Get a free API key:
   - **Alchemy**: https://www.alchemy.com/ (recommended)
   - **Infura**: https://www.infura.io/
   - **Public RPC**: https://rpc.sepolia.org (no API key needed, but less reliable)

### "PRIVATE_KEY not set" Error

**Solution:**
```env
PRIVATE_KEY=your_private_key_without_0x_prefix
```

### "Insufficient funds" Error

**Solution:**
- Get Sepolia ETH from a faucet:
  - https://sepoliafaucet.com/
  - https://faucet.quicknode.com/ethereum/sepolia
  - https://www.alchemy.com/faucets/ethereum-sepolia

### Compilation Errors

**Solution:**
```bash
# Clean and recompile
rm -rf artifacts cache
npx hardhat clean
npx hardhat compile
```

### Network Not Found

**Solution:**
Check `hardhat.config.js` has the network configured. For custom networks, add them to the config.

## Quick Fixes

### Test RPC Connection

```bash
# Test if RPC is working
curl -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  YOUR_RPC_URL
```

### Verify Environment Variables

```bash
# Check if .env is loaded
node -e "require('dotenv').config(); console.log('RPC:', process.env.SEPOLIA_RPC_URL ? 'Set' : 'Missing')"
```

## Still Having Issues?

1. Check Hardhat version: `npx hardhat --version`
2. Check Node version: `node --version` (should be 16+)
3. Reinstall dependencies: `rm -rf node_modules && npm install`
4. Check network status: Visit https://sepolia.etherscan.io/
