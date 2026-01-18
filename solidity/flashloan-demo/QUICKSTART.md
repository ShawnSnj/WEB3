# Quick Start Guide

## Do I Need to Run a Local Node? ❌ NO!

**Short answer:** No! Hardhat handles everything for you.

## How It Works

### Hardhat Fork Mode (Testing)

```
┌─────────────────┐
│  Your Computer  │
│                 │
│  ┌───────────┐  │
│  │  Hardhat  │  │ ← Local simulator (runs on your machine)
│  │   Fork    │  │
│  └─────┬─────┘  │
│        │        │
└────────┼────────┘
         │
         │ Connects via HTTP
         ▼
┌─────────────────┐
│ Remote RPC Node │ ← Alchemy/Infura (you just need API key)
│  (Alchemy/...)  │
└─────────────────┘
         │
         ▼
┌─────────────────┐
│ Ethereum Mainnet│ ← Real blockchain (you read from it)
└─────────────────┘
```

**What happens:**
1. Hardhat connects to remote RPC (Alchemy/Infura)
2. Downloads blockchain state (contracts, balances, etc.)
3. Creates a local copy on your computer
4. You test your contracts locally (free, fast, no gas costs!)
5. You can manipulate state for testing

**You need:**
- ✅ An RPC URL (free from Alchemy/Infura)
- ❌ NO local node
- ❌ NO syncing blockchain
- ❌ NO waiting hours for sync

## Setup Steps

### 1. Get Free RPC URL

**Option A: Alchemy (Recommended)**
1. Go to https://www.alchemy.com/
2. Sign up (free)
3. Create app → Ethereum Mainnet
4. Copy HTTP URL: `https://eth-mainnet.g.alchemy.com/v2/YOUR-KEY`

**Option B: Public RPC (No Signup)**
- Use: `https://eth.llamarpc.com` (rate-limited but works)

### 2. Update hardhat.config.js

```javascript
module.exports = {
  solidity: "0.8.28",
  networks: {
    hardhat: {
      forking: {
        url: "https://eth-mainnet.g.alchemy.com/v2/YOUR-KEY", // ← Paste here
      }
    }
  }
};
```

### 3. Run Scripts

```bash
# Compile contracts
npx hardhat compile

# Run demo (uses fork mode automatically)
npx hardhat run scripts/flashloanLiquidation.js --network hardhat
```

That's it! Hardhat automatically:
- Connects to your RPC
- Downloads state
- Runs tests locally

## Testing vs Production

### Testing (Fork Mode) ✅
```bash
npx hardhat run scripts/deploy.js --network hardhat
```
- Uses fork (local simulation)
- Free (no gas)
- Fast
- Can manipulate state
- Safe (no real transactions)

### Production (Real Network) ⚠️
```bash
npx hardhat run scripts/deploy.js --network mainnet
```
- Deploys to real blockchain
- Costs real ETH for gas
- Permanent transactions
- Need real tokens

## Common Questions

**Q: Do I need to download the entire blockchain?**  
A: No! Hardhat only downloads what it needs when you query it.

**Q: How long does forking take?**  
A: Usually 10-30 seconds on first run, then instant afterwards.

**Q: Can I use my own node?**  
A: Yes! If you run your own node (Geth/Nethermind), you can point Hardhat to `http://localhost:8545`.

**Q: Do I need to sync a node for testing?**  
A: No! Fork mode doesn't require syncing. Only if you want to run your own full node.

## Troubleshooting

**"Error: connect ECONNREFUSED"**
- Check your RPC URL is correct
- Make sure you have internet connection
- Try a different RPC provider

**"Rate limit exceeded"**  
- Sign up for Alchemy/Infura (free tier)
- Or wait a few minutes and try again

**Slow performance**
- First fork can be slow (downloading state)
- Subsequent runs are much faster
- Consider using a dedicated RPC provider
