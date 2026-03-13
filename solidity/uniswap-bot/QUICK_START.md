# Quick Start Guide

## 5-Minute Setup

### 1. Install Dependencies

```bash
cd uniswap-bot
go mod download
```

### 2. Configure Environment

```bash
cp .env.example .env
```

Edit `.env` with your settings:

```env
PRIVATE_KEY=your_private_key_here
RPC_URL=https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY
MONITOR_PAIRS=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2,0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48
ENABLE_AUTO_SWAP=false
```

### 3. Build & Run

```bash
go build -o uniswap-bot .
./uniswap-bot
```

## Example: Monitor WETH/USDC

```env
# Monitor WETH/USDC pair
MONITOR_PAIRS=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2,0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48

# Check prices every 5 seconds
POLL_INTERVAL=5

# Only execute if profit > 1%
MIN_PROFIT_PERCENTAGE=0.01

# Monitor only (don't auto-swap)
ENABLE_AUTO_SWAP=false
```

## Enable Auto-Swap

⚠️ **Test on testnet first!**

```env
ENABLE_AUTO_SWAP=true
MIN_PROFIT_PERCENTAGE=0.02  # 2% minimum profit
MAX_SLIPPAGE_BPS=30         # 0.3% max slippage
```

## Common Token Addresses (Mainnet)

- **WETH**: `0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2`
- **USDC**: `0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48`
- **USDT**: `0xdAC17F958D2ee523a2206206994597C13D831ec7`
- **DAI**: `0x6B175474E89094C44Da98b954EedeAC495271d0F`

## Testnet Setup (Sepolia)

```env
RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_API_KEY
UNISWAP_V2_ROUTER=0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008
```

## Next Steps

1. ✅ Start with `ENABLE_AUTO_SWAP=false` to monitor only
2. ✅ Verify prices are updating correctly
3. ✅ Check for profitable opportunities
4. ✅ Test on Sepolia first
5. ⚠️ Then enable auto-swap on mainnet (with caution!)

See `README.md` for full documentation.
