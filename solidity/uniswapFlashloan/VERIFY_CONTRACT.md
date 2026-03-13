# Verify Contract on Arbiscan

## Your Contract Address

From `deployments/arbitrum.json`:
- **Contract Address**: `0x2c98297d4D81cC8eAf96B600ab0A874590779081`
- **Network**: Arbitrum One
- **Deployer**: `0xFA4eFFDfce5fCe12AD9E2296E1123d4Ce6CFC3b3`

## Verification Command

Replace `CONTRACT_ADDRESS` with your actual address:

```bash
npx hardhat verify --network arbitrum \
  0x2c98297d4D81cC8eAf96B600ab0A874590779081 \
  "0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb" \
  "0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24" \
  "0xE592427A0AEce92De3Edee1F18E0157C05861564" \
  false \
  3000 \
  50
```

## What Each Parameter Means

1. `0x2c98297d4D81cC8eAf96B600ab0A874590779081` - Your contract address
2. `0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb` - Aave Pool Addresses Provider
3. `0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24` - Uniswap V2 Router
4. `0xE592427A0AEce92De3Edee1F18E0157C05861564` - Uniswap V3 Router
5. `false` - Use V3 Router (false = use V2)
6. `3000` - Default pool fee (0.3%)
7. `50` - Max slippage in basis points (0.5%)

## Prerequisites

Make sure your `.env` file has:

```env
ARBISCAN_API_KEY=your_arbiscan_api_key
PRIVATE_KEY=your_private_key
ARBITRUM_RPC_URL=your_rpc_url
```

## Get Arbiscan API Key

1. Go to https://arbiscan.io/
2. Sign up / Log in
3. Go to API Keys section
4. Create new API key
5. Copy and add to `.env`

## Alternative: Verify via Arbiscan Website

1. Go to: https://arbiscan.io/address/0x2c98297d4D81cC8eAf96B600ab0A874590779081
2. Click "Contract" tab
3. Click "Verify and Publish"
4. Fill in:
   - Compiler: `0.8.28`
   - License: `MIT`
   - Optimization: `Yes` (200 runs)
   - Constructor arguments: (use the values from above)
5. Paste your contract code
6. Submit

## Check Verification Status

After running the command, check:
https://arbiscan.io/address/0x2c98297d4D81cC8eAf96B600ab0A874590779081

If verified, you'll see a green checkmark ✓ and be able to read the contract source code.
