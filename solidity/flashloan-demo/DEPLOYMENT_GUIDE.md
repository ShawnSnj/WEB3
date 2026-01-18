# Deployment & Testing Guide

## Quick Answer: Testing vs Production

### For Testing (Hardhat Fork) ✅
**You DON'T need to deploy separately!** The test script deploys automatically.

### For Production (Real Network) ⚠️
**Yes, deploy first**, then use the deployed address.

---

## Testing Workflow (Recommended First)

### Option 1: All-in-One Test Script (Easiest)

```bash
# This script does EVERYTHING automatically:
# 1. Compiles contract
# 2. Deploys to Hardhat fork
# 3. Sets up test scenario
# 4. Runs flash loan liquidation
# 5. Shows results

npx hardhat run scripts/flashloanLiquidation.js --network hardhat
```

**Output:**
```
1. Deploying FlashLoanExample contract...
✓ FlashLoan deployed at: 0x5FbDB2315678afecb367f032d93F642f64180aa3

2. Fetching Aave reserve data...
3. Setting up victim position...
4. Funding contract...
5. Checking health factor...
6. Executing flash loan liquidation...
✓ Transaction confirmed!
7. Checking balances...
8. Withdrawing profits...
✓ Complete!
```

**Advantages:**
- ✅ One command does everything
- ✅ No manual steps
- ✅ Perfect for learning/testing
- ✅ No real transactions (safe)

---

### Option 2: Separate Deploy + Test (More Control)

If you want to deploy and test separately:

```bash
# Step 1: Deploy
npx hardhat run scripts/deploy.js --network hardhat
# Output: Contract deployed at: 0x5FbDB2315678afecb367f032d93F642f64180aa3

# Step 2: Test with deployed address
FLASHLOAN_ADDRESS=0x5FbDB2315678afecb367f032d93F642f64180aa3 \
VICTIM_ADDRESS=0xYourVictimAddress \
npx hardhat run scripts/simpleFlashLoan.js --network hardhat
```

---

## Production Workflow (Real Networks)

### Step 1: Deploy to Real Network

```bash
# Make sure you have network config in hardhat.config.js
npx hardhat run scripts/deploy.js --network mainnet
```

**Save the deployed address!**

### Step 2: Fund the Contract

Send USDC to the contract address to cover flash loan premiums:
- Amount needed: ~0.05-0.09% of max liquidation amount
- Example: For 100,000 USDC liquidations, send ~50-90 USDC

### Step 3: Use Deployed Contract

```bash
# Use the deployed address
FLASHLOAN_ADDRESS=0xYourDeployedAddress \
VICTIM_ADDRESS=0xLiquidatableAddress \
npx hardhat run scripts/simpleFlashLoan.js --network mainnet
```

Or integrate into your Go bot:

```go
// In your Go bot
flashLoanAddress := common.HexToAddress("0xYourDeployedAddress")
// Call requestLiquidationLoan function
```

---

## Comparison Table

| Scenario | Deploy First? | Network | Cost | Use Case |
|----------|--------------|---------|------|----------|
| **Testing (All-in-One)** | ❌ No | Hardhat fork | Free | Learning, testing |
| **Testing (Separate)** | ✅ Yes | Hardhat fork | Free | Manual testing |
| **Production** | ✅ Yes | Mainnet/Arbitrum | Real ETH | Real liquidations |

---

## Recommended Learning Path

### Day 1: Learn & Test
```bash
# Just run this - everything happens automatically
npx hardhat run scripts/flashloanLiquidation.js --network hardhat
```

### Day 2: Understand Deployment
```bash
# Deploy separately to see the process
npx hardhat run scripts/deploy.js --network hardhat

# Then test with the deployed address
FLASHLOAN_ADDRESS=0x... npx hardhat run scripts/simpleFlashLoan.js --network hardhat
```

### Day 3: Production
```bash
# Deploy to real network (careful!)
npx hardhat run scripts/deploy.js --network mainnet

# Use in production
FLASHLOAN_ADDRESS=0x... npx hardhat run scripts/simpleFlashLoan.js --network mainnet
```

---

## Common Questions

**Q: Do I need to deploy every time I test?**  
A: With `flashloanLiquidation.js`, no! It deploys fresh each run.

**Q: Can I reuse a deployed contract?**  
A: Yes! If you deploy separately, save the address and reuse it.

**Q: What about the Go bot?**  
A: For Go bot integration:
1. Deploy Solidity contract once (save address)
2. Update Go bot with contract address
3. Go bot calls the deployed contract

**Q: Does Hardhat fork deployment cost money?**  
A: No! Hardhat fork is completely free - all transactions are simulated locally.

**Q: When should I deploy to a real network?**  
A: Only when:
- ✅ Your contract is thoroughly tested
- ✅ You understand the risks
- ✅ You have funds for gas fees
- ✅ You're ready for production

---

## Quick Commands Cheat Sheet

```bash
# Test everything (recommended for beginners)
npx hardhat run scripts/flashloanLiquidation.js --network hardhat

# Deploy only
npx hardhat run scripts/deploy.js --network hardhat

# Test with existing deployment
FLASHLOAN_ADDRESS=0x... npx hardhat run scripts/simpleFlashLoan.js --network hardhat

# Withdraw profits
FLASHLOAN_ADDRESS=0x... TOKEN_ADDRESS=0x... \
npx hardhat run scripts/withdrawProfit.js --network hardhat
```
