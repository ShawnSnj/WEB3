# Uniswap Flash Loan Arbitrage

A Solidity smart contract for executing arbitrage opportunities using Aave flash loans and Uniswap swaps.

## Features

- 🔄 **Flash Loan Integration**: Uses Aave V3 flash loans for capital-efficient arbitrage
- 🔀 **Uniswap V2/V3 Support**: Supports both Uniswap V2 and V3 routers
- 💰 **Profit Optimization**: Automatically calculates and executes profitable trades
- 🛡️ **Slippage Protection**: Configurable slippage tolerance
- 👤 **Owner Controls**: Owner can withdraw profits and update configuration

## How It Works

1. **Flash Loan Request**: Contract requests a flash loan from Aave
2. **Arbitrage Execution**: Swaps borrowed tokens on Uniswap for profit
3. **Loan Repayment**: Repays flash loan + premium in the same transaction
4. **Profit Retention**: Remaining profit stays in the contract

## Quick Start

### 1. Install Dependencies

```bash
npm install
```

### 2. Configure Environment

Create a `.env` file:

```env
PRIVATE_KEY=your_private_key_here
RPC_URL=https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY
ETHERSCAN_API_KEY=your_etherscan_api_key
```

### 3. Compile

```bash
npx hardhat compile
```

### 4. Deploy

```bash
# Deploy to Sepolia testnet
npx hardhat run scripts/deploy.js --network sepolia

# Deploy to mainnet
npx hardhat run scripts/deploy.js --network mainnet
```

## Contract Functions

### `requestArbitrageLoan`

Execute a simple arbitrage with two tokens:

```solidity
function requestArbitrageLoan(
    address _token,        // Token to borrow
    uint256 _amount,       // Amount to borrow
    address _tokenIn,      // Input token for swap
    address _tokenOut,     // Output token for swap
    uint256 _expectedProfit // Minimum expected profit
) external
```

### `requestArbitrageLoanWithPath`

Execute arbitrage with custom swap path (V2):

```solidity
function requestArbitrageLoanWithPath(
    address _token,
    uint256 _amount,
    address[] memory _swapPath,
    uint256 _expectedProfit
) external
```

### `withdrawProfit`

Withdraw profits from contract (owner only):

```solidity
function withdrawProfit(address token) external onlyOwner
```

### `getBalance`

Check contract balance:

```solidity
function getBalance(address token) external view returns (uint256)
```

## Integration with uniswap-bot

The contract is designed to be called from the `uniswap-bot` Go project:

1. Deploy the contract
2. Set `FLASHLOAN_CONTRACT_ADDRESS` in bot's `.env`
3. Set `USE_FLASHLOAN=true` to enable flash loan arbitrage
4. Bot will automatically use flash loans for large opportunities

## Configuration

### Constructor Parameters

- `_addressesProvider`: Aave Pool Addresses Provider
- `_v2Router`: Uniswap V2 Router address
- `_v3Router`: Uniswap V3 Router address
- `_useV3Router`: Whether to use V3 by default
- `_defaultPoolFee`: Default pool fee for V3 (3000 = 0.3%)
- `_maxSlippageBps`: Maximum slippage in basis points (50 = 0.5%)

### Environment Variables

```env
# Deployment
USE_V3_ROUTER=false
DEFAULT_POOL_FEE=3000
MAX_SLIPPAGE_BPS=50

# Network RPC URLs
RPC_URL=https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY
SEPOLIA_RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_API_KEY
```

## Network Addresses

### Mainnet

- Aave Pool Addresses Provider: `0x2f39d218133AFaB8F2B819B1066c7E434Ad94E9e`
- Uniswap V2 Router: `0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D`
- Uniswap V3 Router: `0xE592427A0AEce92De3Edee1F18E0157C05861564`

### Sepolia

- Aave Pool Addresses Provider: `0x012bAC54348C0E635dCAc19D0f3C925d2C0Cb0e6`
- Uniswap V2 Router: `0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008`
- Uniswap V3 Router: `0x3bFA4769FB09eefC5a80d6E87c3B9C650f7Ae48E`

## Security Considerations

- ⚠️ **Flash loans must be repaid in the same transaction**
- ⚠️ **Test thoroughly on testnets first**
- ⚠️ **Monitor gas costs vs. profits**
- ⚠️ **Set appropriate slippage protection**
- ⚠️ **Use a dedicated wallet for deployment**

## Project Structure

```
uniswapFlashloan/
├── contracts/
│   └── UniswapFlashLoanArbitrage.sol
├── scripts/
│   └── deploy.js
├── hardhat.config.js
├── package.json
└── README.md
```

## License

MIT
