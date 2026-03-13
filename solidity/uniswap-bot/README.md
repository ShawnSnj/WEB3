# Uniswap Price Monitoring & Trading Bot

A Go-based bot for monitoring Uniswap token prices and executing profitable swaps automatically.

## Features

- 📊 **Real-time Price Monitoring**: Continuously monitors token prices in Uniswap pools
- 💰 **Profit Detection**: Automatically detects profitable swap opportunities
- ⚡ **Auto-Swap**: Optional automatic execution of profitable swaps
- 🔒 **Safety Controls**: Configurable slippage, profit thresholds, and swap limits
- 🌐 **Multi-Network**: Supports Ethereum mainnet, testnets, and L2s
- 📈 **Detailed Logging**: Comprehensive price and transaction logs

## Quick Start

### 1. Prerequisites

- Go 1.21+ installed
- Access to an RPC endpoint (Alchemy, Infura, or public)
- Wallet with tokens to trade

### 2. Installation

```bash
cd uniswap-bot

# Install dependencies
go mod download

# Build the bot
go build -o uniswap-bot .
```

### 3. Configuration

Create a `.env` file:

```bash
cp .env.example .env
```

Edit `.env` with your settings:

```env
# Your private key (KEEP SECRET!)
PRIVATE_KEY=your_private_key_here

# RPC endpoint
RPC_URL=https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY

# Uniswap V2 Router (mainnet default)
UNISWAP_V2_ROUTER=0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D

# Token pairs to monitor (comma-separated addresses)
# Format: TOKEN0_ADDRESS,TOKEN1_ADDRESS
MONITOR_PAIRS=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2,0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48
# Example: WETH/USDC

# Polling interval (seconds)
POLL_INTERVAL=5

# Minimum profit percentage (0.01 = 1%)
MIN_PROFIT_PERCENTAGE=0.01

# Maximum slippage (basis points, 50 = 0.5%)
MAX_SLIPPAGE_BPS=50

# Enable auto-swap (true/false)
ENABLE_AUTO_SWAP=false
```

### 4. Run the Bot

```bash
# Start monitoring (auto-swap disabled)
go run .

# Or use the compiled binary
./uniswap-bot
```

## Configuration Options

| Variable | Default | Description |
|----------|---------|-------------|
| `RPC_URL` | - | Ethereum RPC endpoint (required) |
| `PRIVATE_KEY` | - | Your wallet private key (required) |
| `UNISWAP_V2_ROUTER` | `0x7a250d...` | Uniswap V2 Router address |
| `MONITOR_PAIRS` | - | Comma-separated token addresses (required) |
| `POLL_INTERVAL` | `5` | Seconds between price checks |
| `MIN_PROFIT_PERCENTAGE` | `0.01` | Minimum profit to execute (1%) |
| `MAX_SLIPPAGE_BPS` | `50` | Max slippage in basis points (0.5%) |
| `SWAP_AMOUNT` | - | Fixed swap amount (wei), empty = use balance |
| `GAS_PRICE_MULTIPLIER` | `1.1` | Gas price multiplier (10% higher) |
| `ENABLE_AUTO_SWAP` | `false` | Enable automatic swap execution |
| `MIN_SWAP_AMOUNT` | `1000000000000000` | Minimum swap amount (wei) |
| `MAX_SWAP_AMOUNT` | `0` | Maximum swap amount (wei, 0 = no limit) |

## How It Works

### 1. Price Monitoring

The bot continuously:
- Polls Uniswap pools for current prices
- Calculates price ratios between token pairs
- Logs price changes

### 2. Opportunity Detection

For each monitored pair:
- Checks wallet balances for both tokens
- Calculates potential swap outputs
- Compares expected output vs input to find profit
- Only considers opportunities above `MIN_PROFIT_PERCENTAGE`

### 3. Swap Execution (if enabled)

When a profitable opportunity is found:
1. Checks token allowance for Uniswap Router
2. Approves tokens if needed (one-time max approval)
3. Calculates minimum output with slippage protection
4. Executes swap transaction
5. Waits for confirmation
6. Logs results

## Example Output

```
Configuration loaded:
  Wallet: 0x1234...
  Router: 0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D
  RPC: https://eth-mainnet.g.alchemy.com/v2/...

Connected to Ethereum node

Starting price monitoring...
Monitoring 1 token pair(s)
Poll interval: 5s
Min profit: 1.00%
Auto-swap: false

[WETH/USDC] Price: 2500.000000 (1 token0 = 2500.000000 token1)
[WETH/USDC] Price: 2501.000000 (1 token0 = 2501.000000 token1)
💰 PROFITABLE OPPORTUNITY FOUND!
  Pair: WETH/USDC
  Direction: token0_to_token1
  Profit: 1.25%
  Amount in: 1000000000000000000
  Expected out: 1012500000
⚠️  Auto-swap disabled. Set ENABLE_AUTO_SWAP=true to execute
```

## Network Configuration

### Ethereum Mainnet

```env
RPC_URL=https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY
UNISWAP_V2_ROUTER=0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D
MONITOR_PAIRS=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2,0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48
```

### Sepolia Testnet

```env
RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_API_KEY
UNISWAP_V2_ROUTER=0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008
```

### Arbitrum

```env
RPC_URL=https://arb-mainnet.g.alchemy.com/v2/YOUR_API_KEY
UNISWAP_V2_ROUTER=0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24
```

## Common Token Addresses

### Ethereum Mainnet

- **WETH**: `0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2`
- **USDC**: `0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48`
- **USDT**: `0xdAC17F958D2ee523a2206206994597C13D831ec7`
- **DAI**: `0x6B175474E89094C44Da98b954EedeAC495271d0F`

## Safety & Best Practices

### Start with Monitoring Only

```env
ENABLE_AUTO_SWAP=false
```

Monitor prices first to understand behavior before enabling auto-swap.

### Use Testnet First

Test on Sepolia testnet before using mainnet.

### Set Conservative Limits

```env
MIN_PROFIT_PERCENTAGE=0.02  # 2% minimum profit
MAX_SLIPPAGE_BPS=30         # 0.3% max slippage
MAX_SWAP_AMOUNT=1000000000000000000  # 1 ETH max
```

### Use Dedicated Wallet

- Use a separate wallet for the bot
- Only fund with amounts you're comfortable trading
- Monitor regularly

## Troubleshooting

### "Failed to connect to Ethereum node"
- Check your `RPC_URL` is correct
- Verify your RPC provider is working
- Try a different RPC endpoint

### "No profitable opportunities found"
- This is normal - profitable opportunities are rare
- Try lowering `MIN_PROFIT_PERCENTAGE` (but be careful!)
- Check you have sufficient token balances

### "Insufficient balance"
- Ensure your wallet has tokens to swap
- Check token addresses are correct

### "Approval transaction reverted"
- Verify token contract address is correct
- Check you have enough ETH for gas
- Some tokens may have restrictions

### "Swap transaction reverted"
- Price may have changed (slippage)
- Pool may not have enough liquidity
- Try increasing `MAX_SLIPPAGE_BPS`

## Advanced Usage

### Multiple Token Pairs

```env
MONITOR_PAIRS=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2,0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48,0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2,0xdAC17F958D2ee523a2206206994597C13D831ec7
```

### Fixed Swap Amount

```env
SWAP_AMOUNT=1000000000000000000  # Always swap 1 token
```

### Dynamic Swap Amount

Leave `SWAP_AMOUNT` empty to use 50% of available balance.

## Project Structure

```
uniswap-bot/
├── main.go              # Main bot logic
├── contracts/          # Contract bindings
│   ├── erc20.go       # ERC20 token contract
│   └── uniswap.go     # Uniswap V2 Router
├── .env                # Configuration (DO NOT COMMIT)
├── .env.example        # Example configuration
├── .gitignore          # Git ignore rules
├── go.mod              # Go module
└── README.md           # This file
```

## License

MIT

## Disclaimer

This bot is for educational purposes. Trading cryptocurrencies involves risk. Always:
- Test on testnets first
- Start with small amounts
- Monitor the bot closely
- Understand the risks involved
- Never share your private key
