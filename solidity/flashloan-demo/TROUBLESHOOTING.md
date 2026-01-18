# Troubleshooting Guide - Common Errors & Solutions

## 📋 How to Use This Guide

1. **Copy the error message** from your terminal output
2. **Search for it** in this document (Ctrl+F / Cmd+F)
3. **Follow the solution** steps

---

## 🔴 Common Errors & Solutions

### Error: "connect ECONNREFUSED" or "could not detect network"

**What it means:** Hardhat can't connect to your RPC endpoint.

**Solution:**
```javascript
// 1. Check your hardhat.config.js has a valid RPC URL
// 2. Make sure you have internet connection
// 3. Try a different RPC provider:

// Option A: Use public RPC (free, no API key)
url: "https://eth.llamarpc.com"

// Option B: Get free Alchemy account
// Go to https://www.alchemy.com/ → Sign up → Create app → Copy URL
url: "https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY"
```

---

### Error: "Rate limit exceeded" or "429 Too Many Requests"

**What it means:** You've exceeded your RPC rate limit.

**Solutions:**
1. **Wait a few minutes** and try again (for public RPCs)
2. **Sign up for Alchemy/Infura** (free tier = 300M requests/month)
3. **Use a different RPC provider**

---

### Error: "Insufficient balance to repay flash loan"

**What it means:** The contract doesn't have enough USDC to pay the flash loan premium.

**Why:** Flash loans require repayment of loan + premium in the same transaction. The contract needs USDC balance for the premium.

**Solution:**
```bash
# For Hardhat fork testing, the script should handle this automatically
# If you see this error, check the flashloanLiquidation.js script

# For real network:
# 1. Calculate premium needed: ~0.05-0.09% of loan amount
#    Example: 10,000 USDC loan = 5-9 USDC premium
# 2. Send USDC to contract address
```

**Fix in script (flashloanLiquidation.js):**
```javascript
// Around line 60-70, make sure it's funding enough:
const fundingAmount = ethers.parseUnits("10000", 6); // 10,000 USDC (plenty)
```

---

### Error: "Health factor is above 1.0" or "Position not liquidatable"

**What it means:** The victim's position is healthy (HF >= 1.0).

**Solution:**
```javascript
// In flashloanLiquidation.js, the script sets up a victim position
// Make sure this section runs on Hardhat fork:

if (network.name === "hardhat") {
    // This should set victim to be unhealthy
    // Check lines 52-75 in flashloanLiquidation.js
}
```

**For real network:** You need to find addresses with HF < 1.0:
- Use Aave frontend: https://app.aave.com/
- Or use your Go bot to monitor positions

---

### Error: "Transaction reverted" or "execution reverted"

**Common causes:**

1. **Not enough gas**
   ```javascript
   // In your transaction, increase gas limit:
   const tx = await contract.function(...);
   await tx.wait({ gasLimit: 5000000 }); // Increase if needed
   ```

2. **Position already liquidated** (by someone else)
   - Flash loan liquidations are competitive
   - Someone else may have liquidated first

3. **Wrong asset addresses**
   - Check USDC, WETH, Pool addresses are correct for your network
   - Ethereum vs Arbitrum have different addresses

---

### Error: "Cannot read property 'balanceOf' of undefined"

**What it means:** Token contract instance is not created properly.

**Solution:**
```javascript
// Make sure you have the ERC20 ABI or interface
const token = await ethers.getContractAt("IERC20", TOKEN_ADDRESS);

// Or if IERC20 interface doesn't exist:
const tokenABI = ["function balanceOf(address) view returns (uint256)"];
const token = await ethers.getContractAt(tokenABI, TOKEN_ADDRESS);
```

---

### Error: "Contract 'FlashLoanExample' not found"

**What it means:** Contract not compiled or wrong path.

**Solution:**
```bash
# 1. Compile contracts first
npx hardhat compile

# 2. Make sure contract file exists
ls contracts/FlashLoanExample.sol

# 3. Check for compilation errors
npx hardhat compile --force
```

---

### Error: "Invalid address" or "invalid address (argument="address", value=...)"

**What it means:** Address format is wrong.

**Solution:**
```javascript
// Make sure addresses start with 0x and are 42 characters
const address = "0x..." // ✓ Correct
const address = "..."   // ✗ Wrong

// For environment variables:
const address = process.env.ADDRESS || ""; // Add validation
if (!address || !address.startsWith("0x")) {
    throw new Error("Invalid address");
}
```

---

### Error: "nonce too high" or "nonce too low"

**What it means:** Transaction nonce mismatch (usually on real networks).

**Solution:**
```bash
# Reset nonce (for real networks)
# Check your wallet's current nonce on Etherscan

# Or in code:
const nonce = await provider.getTransactionCount(address, "latest");
// Use this nonce in your transaction
```

---

### Error: "insufficient funds for gas" or "out of gas"

**What it means:** Not enough ETH/ARB to pay for gas.

**Solution:**
1. **For Hardhat fork:** Script should give deployer unlimited ETH automatically
2. **For real network:** Fund your wallet with ETH/ARB for gas

---

### Error: "POOL_ADDRESSES_PROVIDER" not found or undefined

**What it means:** Network configuration issue.

**Solution:**
```javascript
// Make sure you're using correct addresses for your network:

// Ethereum Mainnet
const POOL_ADDRESSES_PROVIDER = "0x2f39d218133AFaB8F2B819B1066c7E434Ad94E9e";

// Arbitrum
const POOL_ADDRESSES_PROVIDER = "0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb";

// Check: https://docs.aave.com/developers/deployed-contracts/v3-mainnet
```

---

## 🟡 Warnings (Usually OK)

### Warning: "Using the 'eth' chain will use the default chain..."

**What it means:** Hardhat is using default settings. Usually fine for testing.

**Action:** Can be ignored if everything works.

---

### Warning: "Contract deployed at address with no code"

**What it means:** Contract deployment might have failed or address is wrong.

**Solution:**
```bash
# Check deployment transaction on Etherscan (for real network)
# Or check Hardhat console output for deployment address
```

---

## ✅ Successful Output Example

When everything works, you should see:

```
=== Flash Loan Liquidation Demo ===

Deployer: 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
Balance: 10000.0 ETH

1. Deploying FlashLoanExample contract...
✓ FlashLoan deployed at: 0x5FbDB2315678afecb367f032d93F642f64180aa3

2. Fetching Aave reserve data...
✓ aWETH Address: 0x4d5F47FA6A74757f35C14fD3a6Ef8E3C9BC514E8
✓ USDC Debt Token: 0x72E95b8931767C79bA4EeE721354d6E99a61D004

3. Setting up victim position (Hardhat fork only)...
✓ Victim position configured

4. Funding contract with USDC for flash loan premium...
✓ Contract funded with 10000.0 USDC

5. Checking victim health factor...
   Health Factor: 0.500000 ✓ Position is liquidatable!

6. Executing flash loan liquidation...
   Transaction sent: 0x...
   ✓ Transaction confirmed in block 18000000
   Gas used: 350000

7. Checking contract balances...
   USDC Balance: 9995.0 USDC
   aWETH Balance: 0.05 aWETH
   ✓ Profit captured as aWETH!

8. Withdrawing aWETH profits...
   ✓ Profits withdrawn to deployer wallet

=== Demo Complete ===
```

---

## 📞 Still Having Issues?

1. **Copy the FULL error message** (including stack trace)
2. **Check which script you ran** and on which network
3. **Share:**
   - The exact command you ran
   - The complete output/error
   - Your `hardhat.config.js` (remove API keys!)
   - Which network you're using (Hardhat fork vs Mainnet)

---

## 🔍 Debug Tips

### Enable Verbose Logging

```javascript
// Add to your scripts:
const hre = require("hardhat");
hre.config.verbose = true;
```

### Check Contract State

```javascript
// After deployment, verify contract code exists:
const code = await ethers.provider.getCode(contractAddress);
console.log("Contract code length:", code.length); // Should be > 0
```

### Test RPC Connection

```javascript
// Test if RPC is working:
const blockNumber = await ethers.provider.getBlockNumber();
console.log("Current block:", blockNumber); // Should print a number
```
