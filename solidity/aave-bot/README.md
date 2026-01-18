# Aave Liquidation Bot

A Go-based bot for monitoring Aave positions and executing liquidations on Arbitrum and Ethereum.

## Features

- 🔍 **Real-time Monitoring**: Continuously monitors health factors of Aave positions
- 🎯 **Event-Driven Discovery**: Automatically discovers addresses with Aave positions via event scanning
- ⚡ **Auto-Liquidation**: Optional automatic execution of liquidation transactions
- 🔒 **Safety Controls**: Configurable thresholds and maximum liquidation amounts
- 🌐 **Multi-Network**: Supports Arbitrum and Ethereum mainnet
- 📊 **Detailed Logging**: Comprehensive transaction and monitoring logs

## Quick Start

### 1. Prerequisites

- Go 1.16+ installed
- `abigen` tool (from go-ethereum): `go install github.com/ethereum/go-ethereum/cmd/abigen@latest`
- Access to an RPC endpoint (Alchemy, Infura, or public)

### 2. Installation

```bash
# Clone or navigate to the project
cd aave-bot

# Install dependencies
go mod download

# Build the bot
go build -o aave-bot main.go
```

### 3. Configuration

Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
```

Edit `.env` with your settings:

```env
# Your private key (KEEP SECRET!)
PRIVATE_KEY=your_private_key_here

# Network: arbitrum or ethereum
NETWORK=arbitrum

# RPC endpoint
RPC_URL=https://arb1.arbitrum.io/rpc

# Aave V3 Pool address (already configured for Arbitrum)
POOL_ADDRESS=0x794a61358D6845594F94dc1DB02A252b5b4814aD

# Enable/disable auto-liquidation
ENABLE_AUTO_LIQUIDATION=false
```

### 4. Run the Bot

```bash
# Start monitoring (auto-liquidation disabled)
go run main.go

# Or use the compiled binary
./aave-bot
```

## Network Configuration

### Arbitrum Mainnet (Default)

```env
NETWORK=arbitrum
RPC_URL=https://arb1.arbitrum.io/rpc
POOL_ADDRESS=0x794a61358D6845594F94dc1DB02A252b5b4814aD
HISTORICAL_BLOCKS_LOOKBACK=5000

# Arbitrum token addresses
DEFAULT_DEBT_ASSET=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1  # WETH
DEFAULT_COLLATERAL_ASSET=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1
```

**Popular Arbitrum Tokens:**
- WETH: `0x82aF49447D8a07e3bd95BD0d56f35241523fBab1`
- USDC (native): `0xaf88d065e77c8cC2239327C5EDb3A432268e5831`
- USDC.e (bridged): `0xFF970A61A04b1cA14834A43f5dE4533eBDDB5CC8`
- USDT: `0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9`
- DAI: `0xDA10009cBd5D07dd0CeCc66161FC93D7c9000da1`
- ARB: `0x912CE59144191C1204E64559FE8253a0e49E6548`

### Ethereum Mainnet

```env
NETWORK=ethereum
RPC_URL=https://eth.llamarpc.com
POOL_ADDRESS=0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2
HISTORICAL_BLOCKS_LOOKBACK=1000

# Ethereum token addresses
DEFAULT_DEBT_ASSET=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2  # WETH
DEFAULT_COLLATERAL_ASSET=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2
```

**Popular Ethereum Tokens:**
- WETH: `0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2`
- USDC: `0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48`
- USDT: `0xdAC17F958D2ee523a2206206994597C13D831ec7`
- DAI: `0x6B175474E89094C44Da98b954EedeAC495271d0F`

### RPC Providers

**Arbitrum:**
- Public: `https://arb1.arbitrum.io/rpc`
- Alchemy: `https://arb-mainnet.g.alchemy.com/v2/YOUR-API-KEY`
- Infura: `https://arbitrum-mainnet.infura.io/v3/YOUR-PROJECT-ID`

**Ethereum:**
- Public: `https://eth.llamarpc.com`
- Alchemy: `https://eth-mainnet.g.alchemy.com/v2/YOUR-API-KEY`
- Infura: `https://mainnet.infura.io/v3/YOUR-PROJECT-ID`

## Configuration Options

### Monitoring Settings

```env
# How often to check health factors (seconds)
POLL_INTERVAL=5

# Number of historical blocks to scan for addresses
# Arbitrum: 5000 blocks ≈ 20 minutes
# Ethereum: 1000 blocks ≈ 3 hours
HISTORICAL_BLOCKS_LOOKBACK=5000
```

### Liquidation Settings

```env
# Enable automatic liquidation execution
ENABLE_AUTO_LIQUIDATION=false

# Minimum profit threshold (0.01 = 1%)
LIQUIDATION_PROFIT_THRESHOLD=0.01

# Maximum debt to cover per liquidation (in wei)
# 1000000000000000000 = 1 token
MAX_LIQUIDATION_AMOUNT=1000000000000000000

# Token addresses for liquidation
DEFAULT_DEBT_ASSET=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1
DEFAULT_COLLATERAL_ASSET=0x82aF49447D8a07e3bd95BD0d56f35241523fBab1
```

## How It Works

### 1. Address Discovery

The bot uses a hybrid approach to find Aave positions:

**Historical Scan (once at startup):**
- Scans the last N blocks for Aave events (Supply, Borrow, Withdraw, Repay)
- Extracts user addresses from these events
- Adds them to the monitoring list

**Real-Time Discovery (continuous):**
- Polls for new events every 10 seconds
- Discovers new addresses as they interact with Aave
- Automatically adds new positions to monitoring

### 2. Health Factor Monitoring

**Every 5 seconds:**
- Calls `getUserAccountData()` for each monitored address
- Converts health factor from ray format (1e27 = 1.0) to float
- Checks if health factor < 1.0 (liquidation threshold)

**Health Factor Calculation:**
```
HF = (Collateral * Liquidation Threshold) / Total Debt

HF > 1.0 = Healthy position
HF < 1.0 = Liquidatable position
```

### 3. Liquidation Execution

When `ENABLE_AUTO_LIQUIDATION=true` and HF < 1.0:

1. Calculate debt to cover (50% of total debt, capped by MAX_LIQUIDATION_AMOUNT)
2. Check bot's token balance
3. Check token allowance for Aave Pool
4. Approve tokens if needed (one-time max approval)
5. Execute `liquidationCall()` transaction
6. Wait for confirmation
7. Log results

## Output Examples

### Monitoring (Auto-Liquidation Disabled)

```
Configuration loaded:
  Pool Address: 0x794a61358D6845594F94dc1DB02A252b5b4814aD
  Poll Interval: 5s
  Historical Lookback: 5000 blocks
  Auto-Liquidation: false
Connected to Ethereum node: https://arb1.arbitrum.io/rpc
Bot wallet address: 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266

PHASE 1: Discovering addresses from historical events
Scanning historical events from block 270000000 to 270005000
Found 42 unique addresses with Aave positions

PHASE 2: Starting health factor monitoring
Checking health factors for 42 addresses...
  0x1234abcd...: HF=1.2345 ✓
  0x5678ef00...: HF=0.8923 ✗

========================================
LIQUIDATION OPPORTUNITY DETECTED!
========================================
User Address: 0x5678ef0012345678abcdef0123456789abcdef01
Health Factor: 0.892300 (Below 1.0 threshold!)
Total Collateral: 10000000000000000000
Total Debt: 9500000000000000000
Available Borrows: 500000000000000000
Block Number: 270005123
Transaction preparation data:
  - Liquidator should call: liquidationCall()
  - User to liquidate: 0x5678ef0012345678abcdef0123456789abcdef01
  - Max debt to cover: ~50% of total debt = 4750000000000000000
========================================

Auto-liquidation disabled. Skipping execution.
```

### Liquidation Execution (Auto-Liquidation Enabled)

```
========================================
LIQUIDATION OPPORTUNITY DETECTED!
========================================
...

Auto-liquidation enabled, executing liquidation...
Preparing liquidation transaction...
  Debt to cover: 4750000000000000000
  Debt asset: 0x82aF49447D8a07e3bd95BD0d56f35241523fBab1
  Collateral asset: 0x82aF49447D8a07e3bd95BD0d56f35241523fBab1
  Bot balance: 10000000000000000000 ✓
  Sufficient allowance already granted ✓
  Executing liquidation...
  Liquidation tx sent: 0xabcd1234...
  Waiting for confirmation...

========================================
LIQUIDATION SUCCESSFUL! 🎉
========================================
Transaction hash: 0xabcd12345678...
Block number: 270005456
Gas used: 350000
========================================
```

## Safety & Security

⚠️ **Important Security Notes:**

1. **Private Keys**: Never commit your `.env` file or share your private key
2. **Testing**: Test with small amounts and `ENABLE_AUTO_LIQUIDATION=false` first
3. **Funding**: Ensure your wallet has sufficient tokens for liquidations
4. **Gas**: Monitor gas prices on Arbitrum (usually low, but can spike)
5. **Limits**: Use `MAX_LIQUIDATION_AMOUNT` to cap exposure per transaction

### Best Practices

- Start with monitoring only (`ENABLE_AUTO_LIQUIDATION=false`)
- Use a dedicated wallet for the bot with limited funds
- Monitor bot logs regularly
- Set conservative `MAX_LIQUIDATION_AMOUNT` values
- Keep your RPC provider credentials secure

## Troubleshooting

### "Failed to connect to Ethereum node"
- Check your RPC_URL is correct
- Verify your RPC provider is working
- Try a different RPC endpoint

### "Failed to instantiate Pool contract"
- Verify POOL_ADDRESS is correct for your network
- Check you're connected to the right network (Arbitrum vs Ethereum)

### "Insufficient balance"
- Ensure your wallet has the debt asset tokens
- Check DEFAULT_DEBT_ASSET address is correct

### "Approval transaction reverted"
- Verify token contract address is correct
- Check you have enough ETH/ARB for gas

### Historical scan failed
- This is normal if no events found in recent blocks
- Real-time discovery will continue working
- Try increasing HISTORICAL_BLOCKS_LOOKBACK

## Development

### Regenerating Contract Bindings

If you need to update the ABI or add new contract methods:

```bash
# Regenerate Pool contract bindings
abigen --abi abis/Pool.json --pkg contracts --type IPool --out contracts/pool.go

# Regenerate ERC20 contract bindings
abigen --abi abis/ERC20.json --pkg contracts --type ERC20 --out contracts/erc20.go
```

### Project Structure

```
aave-bot/
├── main.go              # Main bot logic
├── contracts/           # Auto-generated contract bindings
│   ├── pool.go         # Aave Pool contract
│   └── erc20.go        # ERC20 token contract
├── abis/               # Contract ABIs
│   ├── Pool.json       # Aave Pool ABI
│   └── ERC20.json      # ERC20 ABI
├── .env                # Configuration (DO NOT COMMIT)
├── .env.example        # Example configuration
├── .gitignore          # Git ignore rules
└── README.md           # This file
```

## License

MIT License - use at your own risk

## Disclaimer

This bot interacts with real financial protocols and executes transactions with real funds. Use at your own risk. The authors are not responsible for any losses incurred through the use of this software. Always test thoroughly with small amounts before deploying with significant capital.
