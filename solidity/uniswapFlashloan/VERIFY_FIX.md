# Fixed: Contract Verification Issue

## Problem
Hardhat was looking for `arbitrumOne` in the etherscan config, but it was set as `arbitrum`.

## Solution
Updated `hardhat.config.js` to use `arbitrumOne` (which is what Hardhat expects).

## Next Steps

### 1. Make sure your `.env` has the API key:

```env
ARBISCAN_API_KEY=your_arbiscan_api_key_here
```

**Get Arbiscan API Key:**
1. Go to https://arbiscan.io/
2. Sign up / Log in
3. Click your profile → API Keys
4. Create new API key
5. Copy and paste into `.env`

### 2. Run verification again:

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

### 3. Check result:

After running, check your contract on Arbiscan:
https://arbiscan.io/address/0x2c98297d4D81c8eAf96B600ab0A874590779081

If verified successfully, you'll see:
- ✅ Green checkmark
- Source code visible
- "Contract" tab shows verified status

## What Changed

**Before:**
```javascript
arbitrum: process.env.ARBISCAN_API_KEY,
```

**After:**
```javascript
arbitrumOne: process.env.ARBISCAN_API_KEY, // Hardhat expects this
arbitrum: process.env.ARBISCAN_API_KEY,    // Keep for compatibility
```

Now Hardhat will find the API key correctly!
