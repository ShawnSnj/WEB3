# Aave Flash Loan Liquidation Demo

A complete demonstration of using Aave flash loans to execute liquidations without requiring upfront capital.

## Overview

This project shows how to:
1. **Borrow funds via flash loan** - Get instant liquidity without collateral
2. **Execute liquidation** - Liquidate unhealthy Aave positions
3. **Repay flash loan** - Pay back the loan + premium in the same transaction
4. **Capture profit** - Keep the liquidation bonus as profit

## How Flash Loan Liquidations Work

```
1. Request Flash Loan (e.g., 10,000 USDC)
   ↓
2. Use borrowed USDC to liquidate a position
   ↓
3. Receive collateral (e.g., aWETH) + liquidation bonus
   ↓
4. Repay flash loan (10,000 USDC + ~0.05% premium)
   ↓
5. Profit = Liquidation bonus - Flash loan premium
```

## Prerequisites

- Node.js and npm installed
- Access to Ethereum mainnet RPC (Alchemy, Infura, etc.)
- Basic understanding of Aave protocol

## Installation

```bash
# Install dependencies
npm install

# Compile contracts
npx hardhat compile
```

## Configuration

### Option 1: Hardhat Fork (Recommended for Testing - NO Local Node Needed!)

Hardhat "forking" creates a **local simulation** that copies blockchain state from a remote RPC. 
**You don't need to run your own node** - Hardhat handles everything!

Update `hardhat.config.js` with your RPC URL:

```javascript
module.exports = {
  solidity: "0.8.28",
  networks: {
    hardhat: {
      forking: {
        url: "https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY",
        // Optional: fork from a specific block
        // blockNumber: 18000000
      }
    }
  }
};
```

**How it works:**
- Hardhat connects to a remote RPC (Alchemy/Infura) 
- Downloads blockchain state to your computer
- Creates a local simulation you can test on
- All transactions happen locally (free!)
- You can manipulate state (set balances, etc.) for testing

**Get a free RPC URL:**
- Alchemy: https://www.alchemy.com/ (free tier: 300M compute units/month)
- Infura: https://www.infura.io/ (free tier available)
- Public RPC: `https://eth.llamarpc.com` (no API key needed, but rate-limited)

### Option 2: Deploy to Real Network (Production)

For production/testing on real networks, add network configuration:

```javascript
module.exports = {
  solidity: "0.8.28",
  networks: {
    hardhat: {
      forking: {
        url: "https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY",
      }
    },
    mainnet: {
      url: "https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY",
      accounts: [process.env.PRIVATE_KEY], // Never commit this!
      chainId: 1
    },
    arbitrum: {
      url: "https://arb1.arbitrum.io/rpc",
      accounts: [process.env.PRIVATE_KEY],
      chainId: 42161
    }
  }
};
```

**Create a `.env` file** (add to `.gitignore`):
```env
PRIVATE_KEY=your_private_key_here
ALCHEMY_API_KEY=your_alchemy_key_here
```

### Network Addresses

**Ethereum Mainnet:**
- Aave Pool: `0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2`
- Pool Addresses Provider: `0x2f39d218133AFaB8F2B819B1066c7E434Ad94E9e`
- WETH: `0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2`
- USDC: `0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48`

**Arbitrum One:**
- Aave Pool: `0x794a61358D6845594F94dc1DB02A252b5b4814aD`
- Pool Addresses Provider: `0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb`
- WETH: `0x82aF49447D8a07e3bd95BD0d56f35241523fBab1`
- USDC: `0xaf88d065e77c8cC2239327C5EDb3A432268e5831`

## Usage

### 1. Deploy the Contract

```bash
# Deploy to local hardhat fork
npx hardhat run scripts/deploy.js --network hardhat

# Or deploy to a real network (requires network config and private key)
npx hardhat run scripts/deploy.js --network mainnet
```

### 2. Run Complete Demo (Hardhat Fork)

```bash
npx hardhat run scripts/flashloanLiquidation.js --network hardhat
```

This script will:
- Deploy the flash loan contract
- Set up a victim position (simulated)
- Fund the contract with USDC for premium
- Execute flash loan liquidation
- Withdraw profits

### 3. Manual Execution

For production use, you'll need to:

1. **Fund the contract** with tokens to cover the flash loan premium:
   - Premium is typically 0.05% - 0.09% of loan amount
   - Example: For 10,000 USDC loan, you need ~5-9 USDC for premium

2. **Call the liquidation function**:
```javascript
await flashLoanContract.requestLiquidationLoanWithWETH(
    USDC_ADDRESS,        // Debt token
    debtAmount,          // Amount to cover (in wei)
    victimAddress        // Address to liquidate
);
```

3. **Withdraw profits**:
```javascript
// Withdraw aTokens directly
await flashLoanContract.withdrawAToken(aWETH_ADDRESS);

// Or withdraw any token
await flashLoanContract.withdraw(TOKEN_ADDRESS);
```

## Contract Functions

### `requestLiquidationLoanWithWETH`
Convenience function that uses WETH as collateral:
```solidity
function requestLiquidationLoanWithWETH(
    address _token,      // Debt token (e.g., USDC)
    uint256 _amount,     // Debt amount to cover
    address _victim      // Address to liquidate
) external
```

### `requestLiquidationLoan`
Full control with custom collateral asset:
```solidity
function requestLiquidationLoan(
    address _token,
    uint256 _amount,
    address _victim,
    address _collateralAsset  // Collateral token (e.g., WETH)
) public
```

### `withdraw`
Withdraw any ERC20 token from the contract:
```solidity
function withdraw(address _tokenAddress) external
```

### `withdrawAToken`
Withdraw aToken and automatically convert to underlying:
```solidity
function withdrawAToken(address _aTokenAddress) external
```

## Important Notes

### ⚠️ Current Limitation

The current implementation requires the contract to be **pre-funded** with the debt token (e.g., USDC) to repay the flash loan premium. 

**Why?** After liquidation, you receive collateral (aWETH), but need to repay the flash loan in the debt token (USDC) in the same transaction. To make this fully automatic, you would need to integrate a DEX swap (e.g., Uniswap) to convert collateral to debt token within `executeOperation`.

### 💡 Solutions

1. **Pre-fund contract** (current approach):
   - Send enough USDC to contract to cover premium
   - Works for demo and simple use cases

2. **Integrate DEX swap** (production approach):
   - Use Uniswap V3 or other DEX within `executeOperation`
   - Swap received collateral → debt token
   - Repay flash loan automatically
   - No pre-funding needed

3. **Use collateral as repayment**:
   - Some protocols allow using received collateral directly
   - Would require protocol-specific implementation

## Security Considerations

- ⚠️ **Test thoroughly** on testnets before mainnet
- ⚠️ **Flash loan premium** must be paid - ensure contract has funds
- ⚠️ **Liquidation must be profitable** - bonus > premium + gas
- ⚠️ **Front-running** - Liquidation opportunities can be competed over
- ⚠️ **Gas costs** - Monitor gas prices for profitability

## Project Structure

```
flashloan-demo/
├── contracts/
│   └── FlashLoanExample.sol    # Main flash loan contract
├── scripts/
│   ├── deploy.js                # Deploy contract
│   ├── flashloanLiquidation.js  # Complete demo script
│   └── simulateLiquidation.js   # Alternative test script
├── hardhat.config.js            # Hardhat configuration
└── README.md                    # This file
```

## Troubleshooting

### "Insufficient balance to repay flash loan"
- Contract needs USDC balance to cover premium
- Premium ≈ 0.05-0.09% of loan amount
- Solution: Fund the contract with USDC

### "Health factor is above 1.0"
- Position is not liquidatable
- Need to find positions with HF < 1.0
- Check on Aave frontend or use monitoring tools

### Transaction reverts
- Check victim address has liquidatable position
- Verify debt amount is not too high
- Ensure contract has enough balance for premium
- Check gas limits are sufficient

## License

MIT License - Use at your own risk

## Disclaimer

This is for educational purposes only. Flash loan liquidations involve financial risk. Always test thoroughly and understand the mechanics before using with real funds.
