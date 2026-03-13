# Quick Start: Uniswap Flash Loan Arbitrage

## 5-Minute Setup

### 1. Install Dependencies

```bash
cd uniswapFlashloan
npm install
```

### 2. Configure Environment

Create `.env`:

```env
PRIVATE_KEY=your_private_key_here
RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_API_KEY
```

### 3. Compile & Deploy

```bash
# Compile
npx hardhat compile

# Deploy to Sepolia
npx hardhat run scripts/deploy.js --network sepolia
```

### 4. Integrate with Bot

Add to `uniswap-bot/.env`:

```env
USE_FLASHLOAN=true
FLASHLOAN_CONTRACT_ADDRESS=0x...  # From deployment output
```

### 5. Run Bot

```bash
cd ../uniswap-bot
go run .
```

## What It Does

The contract enables **capital-efficient arbitrage**:

1. **Borrows** tokens via Aave flash loan (no upfront capital)
2. **Swaps** on Uniswap for profit
3. **Repays** loan + premium automatically
4. **Keeps** profit in contract

## Key Benefits

- ✅ **No Capital Required**: Borrow what you need
- ✅ **Larger Trades**: Execute bigger opportunities
- ✅ **Automatic**: Repayment happens atomically
- ✅ **Profitable**: Only executes if profit > premium

## Example Flow

```
Opportunity: 10 ETH → 25,000 USDC (2.5% profit)

1. Flash loan: Borrow 10 ETH
2. Swap: 10 ETH → 25,000 USDC
3. Repay: 10 ETH + 0.005 ETH (premium)
4. Profit: ~12.5 USDC (stored in contract)
```

## Next Steps

1. ✅ Deploy contract on testnet
2. ✅ Configure bot with contract address
3. ✅ Test with small amounts
4. ✅ Monitor profits
5. ⚠️ Deploy to mainnet (with caution!)

See `README.md` and `../uniswap-bot/FLASHLOAN_INTEGRATION.md` for details.
