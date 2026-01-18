# How to Find Victim Addresses for Liquidation

A **victim** is an address with an unhealthy Aave position (Health Factor < 1.0) that can be liquidated. This guide shows multiple methods to find them.

---

## 🎯 Quick Answer

### For Testing (Hardhat Fork)
**You don't need to find victims!** The test script creates one automatically:
```javascript
// In flashloanLiquidation.js (line 95)
const victim = deployer.address; // Uses deployer as victim for demo
```

### For Real Networks
You need to find actual addresses with unhealthy positions. See methods below.

---

## 📋 Method 1: Using Aave Frontend (Easiest)

### Steps:
1. Go to [Aave App](https://app.aave.com/)
2. Navigate to the **Dashboard** or **Markets** page
3. Look for positions with **Health Factor < 1.0**
4. Copy the wallet address

**Limitations:**
- Only shows positions that have interacted recently
- May not show all liquidatable positions
- Requires manual checking

---

## 📋 Method 2: Event Scanning (Most Reliable)

Scan Aave events to find addresses with positions, then check their health factors.

### JavaScript Example:

```javascript
const { ethers } = require("hardhat");

async function findVictims() {
    const AAVE_POOL = "0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2"; // Ethereum Mainnet
    
    // Aave event signatures
    const supplyEvent = "event Supply(address indexed reserve, address indexed user, address indexed onBehalfOf, uint256 amount, uint16 referralCode)";
    const borrowEvent = "event Borrow(address indexed reserve, address indexed user, address indexed onBehalfOf, uint256 amount, uint8 interestRateMode, uint16 referralCode, address indexed referral)";
    
    const pool = await ethers.getContractAt([supplyEvent, borrowEvent], AAVE_POOL);
    
    // Scan last 1000 blocks for Supply/Borrow events
    const currentBlock = await ethers.provider.getBlockNumber();
    const fromBlock = currentBlock - 1000;
    
    console.log(`Scanning blocks ${fromBlock} to ${currentBlock}...`);
    
    // Get all Supply events
    const supplyFilter = pool.filters.Supply();
    const supplyEvents = await pool.queryFilter(supplyFilter, fromBlock, currentBlock);
    
    // Get all Borrow events
    const borrowFilter = pool.filters.Borrow();
    const borrowEvents = await pool.queryFilter(borrowFilter, fromBlock, currentBlock);
    
    // Extract unique addresses
    const addresses = new Set();
    supplyEvents.forEach(e => addresses.add(e.args.user));
    supplyEvents.forEach(e => addresses.add(e.args.onBehalfOf));
    borrowEvents.forEach(e => addresses.add(e.args.user));
    borrowEvents.forEach(e => addresses.add(e.args.onBehalfOf));
    
    console.log(`Found ${addresses.size} unique addresses\n`);
    
    // Check health factors
    const poolABI = [
        "function getUserAccountData(address user) view returns (uint256 totalCollateralBase, uint256 totalDebtBase, uint256 availableBorrowsBase, uint256 currentLiquidationThreshold, uint256 ltv, uint256 healthFactor)"
    ];
    const poolContract = await ethers.getContractAt(poolABI, AAVE_POOL);
    
    const victims = [];
    for (const addr of addresses) {
        try {
            const userData = await poolContract.getUserAccountData(addr);
            const healthFactor = userData.healthFactor;
            const hfFloat = Number(ethers.formatUnits(healthFactor, 27));
            
            if (hfFloat < 1.0 && hfFloat > 0) {
                console.log(`✓ Victim found: ${addr}`);
                console.log(`  Health Factor: ${hfFloat.toFixed(4)}`);
                console.log(`  Collateral: ${ethers.formatUnits(userData.totalCollateralBase, 8)}`);
                console.log(`  Debt: ${ethers.formatUnits(userData.totalDebtBase, 8)}\n`);
                victims.push({
                    address: addr,
                    healthFactor: hfFloat,
                    collateral: userData.totalCollateralBase,
                    debt: userData.totalDebtBase
                });
            }
        } catch (error) {
            // Skip addresses that error (no position, etc.)
        }
    }
    
    return victims;
}

findVictims().then(victims => {
    console.log(`\nTotal victims found: ${victims.length}`);
    process.exit(0);
}).catch(console.error);
```

**Save as:** `scripts/findVictims.js`

**Run:**
```bash
npx hardhat run scripts/findVictims.js --network mainnet
```

---

## 📋 Method 3: Using Aave Subgraph (Fastest for Historical Data)

Aave has a public subgraph that tracks all positions. Query it via GraphQL:

**📖 For detailed subgraph guide, see [AAVE_SUBGRAPH_GUIDE.md](./AAVE_SUBGRAPH_GUIDE.md)**

### GraphQL Query:

```graphql
{
  userAccounts(
    where: {
      healthFactor_lt: "1.0"
      healthFactor_gt: "0"
    }
    first: 100
  ) {
    id
    healthFactor
    totalCollateralUSD
    totalDebtUSD
  }
}
```

### JavaScript Example:

```javascript
const axios = require("axios");

async function findVictimsFromSubgraph() {
    // Aave V3 Ethereum Subgraph
    const subgraphUrl = "https://api.thegraph.com/subgraphs/name/aave/aave-v3-ethereum";
    
    const query = `
        {
            userAccounts(
                where: {
                    healthFactor_lt: "1.0"
                    healthFactor_gt: "0"
                }
                first: 100
            ) {
                id
                healthFactor
                totalCollateralUSD
                totalDebtUSD
            }
        }
    `;
    
    try {
        const response = await axios.post(subgraphUrl, { query });
        const victims = response.data.data.userAccounts;
        
        console.log(`Found ${victims.length} liquidatable positions:\n`);
        victims.forEach(v => {
            console.log(`Address: ${v.id}`);
            console.log(`  Health Factor: ${v.healthFactor}`);
            console.log(`  Collateral: $${v.totalCollateralUSD}`);
            console.log(`  Debt: $${v.totalDebtUSD}\n`);
        });
        
        return victims.map(v => v.id);
    } catch (error) {
        console.error("Subgraph query failed:", error.message);
        return [];
    }
}

findVictimsFromSubgraph();
```

**Note:** Subgraph may have a delay (few minutes) from on-chain state.

---

## 📋 Method 4: Using Your Go Bot (If You Have One)

If you have the `aave-bot` project, it already discovers and monitors addresses:

```bash
# Run your Go bot in monitoring mode
cd ../aave-bot
go run main.go

# It will:
# 1. Scan historical events for addresses
# 2. Monitor health factors continuously
# 3. Log liquidatable positions
```

The bot outputs addresses with HF < 1.0 that you can use.

---

## 📋 Method 5: Manual Checking (For Specific Addresses)

If you have a specific address you want to check:

```javascript
const { ethers } = require("hardhat");

async function checkHealthFactor(address) {
    const AAVE_POOL = "0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2";
    
    const poolABI = [
        "function getUserAccountData(address user) view returns (uint256 totalCollateralBase, uint256 totalDebtBase, uint256 availableBorrowsBase, uint256 currentLiquidationThreshold, uint256 ltv, uint256 healthFactor)"
    ];
    
    const pool = await ethers.getContractAt(poolABI, AAVE_POOL);
    const userData = await pool.getUserAccountData(address);
    
    const healthFactor = Number(ethers.formatUnits(userData.healthFactor, 27));
    
    console.log(`Address: ${address}`);
    console.log(`Health Factor: ${healthFactor.toFixed(4)}`);
    console.log(`Liquidatable: ${healthFactor < 1.0 ? "YES ✓" : "NO ✗"}`);
    console.log(`Collateral: ${ethers.formatUnits(userData.totalCollateralBase, 8)}`);
    console.log(`Debt: ${ethers.formatUnits(userData.totalDebtBase, 8)}`);
    
    return healthFactor < 1.0;
}

// Check a specific address
checkHealthFactor("0xYourAddressHere");
```

---

## 🔧 Complete Script: Find and Use Victims

Here's a complete script that finds victims and can be integrated with your liquidation script:

```javascript
const { ethers } = require("hardhat");

const AAVE_POOL = "0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2";

async function findAndCheckVictims() {
    console.log("=== Finding Liquidatable Positions ===\n");
    
    // Method: Scan recent events
    const currentBlock = await ethers.provider.getBlockNumber();
    const fromBlock = currentBlock - 5000; // Last ~5000 blocks
    
    const poolABI = [
        "event Supply(address indexed reserve, address indexed user, address indexed onBehalfOf, uint256 amount, uint16 referralCode)",
        "event Borrow(address indexed reserve, address indexed user, address indexed onBehalfOf, uint256 amount, uint8 interestRateMode, uint16 referralCode, address indexed referral)",
        "function getUserAccountData(address user) view returns (uint256 totalCollateralBase, uint256 totalDebtBase, uint256 availableBorrowsBase, uint256 currentLiquidationThreshold, uint256 ltv, uint256 healthFactor)"
    ];
    
    const pool = await ethers.getContractAt(poolABI, AAVE_POOL);
    
    // Get events
    const supplyFilter = pool.filters.Supply();
    const borrowFilter = pool.filters.Borrow();
    
    const [supplyEvents, borrowEvents] = await Promise.all([
        pool.queryFilter(supplyFilter, fromBlock, currentBlock),
        pool.queryFilter(borrowFilter, fromBlock, currentBlock)
    ]);
    
    // Collect unique addresses
    const addresses = new Set();
    [...supplyEvents, ...borrowEvents].forEach(e => {
        if (e.args.user) addresses.add(e.args.user);
        if (e.args.onBehalfOf) addresses.add(e.args.onBehalfOf);
    });
    
    console.log(`Scanned ${currentBlock - fromBlock} blocks`);
    console.log(`Found ${addresses.size} addresses with Aave activity\n`);
    console.log("Checking health factors...\n");
    
    // Check each address
    const victims = [];
    let checked = 0;
    
    for (const addr of addresses) {
        checked++;
        if (checked % 10 === 0) {
            console.log(`Checked ${checked}/${addresses.size} addresses...`);
        }
        
        try {
            const userData = await pool.getUserAccountData(addr);
            const hf = Number(ethers.formatUnits(userData.healthFactor, 27));
            
            if (hf > 0 && hf < 1.0) {
                const collateral = ethers.formatUnits(userData.totalCollateralBase, 8);
                const debt = ethers.formatUnits(userData.totalDebtBase, 8);
                
                console.log(`\n✓ LIQUIDATABLE POSITION FOUND:`);
                console.log(`  Address: ${addr}`);
                console.log(`  Health Factor: ${hf.toFixed(4)}`);
                console.log(`  Collateral: ${collateral}`);
                console.log(`  Debt: ${debt}`);
                
                victims.push({
                    address: addr,
                    healthFactor: hf,
                    collateral: userData.totalCollateralBase,
                    debt: userData.totalDebtBase
                });
            }
        } catch (error) {
            // Skip errors (no position, etc.)
        }
    }
    
    console.log(`\n=== Summary ===`);
    console.log(`Total addresses checked: ${checked}`);
    console.log(`Liquidatable positions found: ${victims.length}`);
    
    if (victims.length > 0) {
        console.log(`\nVictim addresses:`);
        victims.forEach((v, i) => {
            console.log(`${i + 1}. ${v.address} (HF: ${v.healthFactor.toFixed(4)})`);
        });
    }
    
    return victims;
}

// Export for use in other scripts
module.exports = { findAndCheckVictims };

// Run if called directly
if (require.main === module) {
    findAndCheckVictims()
        .then(() => process.exit(0))
        .catch(error => {
            console.error(error);
            process.exit(1);
        });
}
```

**Save as:** `scripts/findVictims.js`

**Run:**
```bash
npx hardhat run scripts/findVictims.js --network mainnet
```

---

## 🎯 Using Victim Address in Your Script

Once you have a victim address, use it in your liquidation script:

```javascript
// Option 1: Hardcode it
const VICTIM_ADDRESS = "0x1234...abcd";

// Option 2: Pass as environment variable
const VICTIM_ADDRESS = process.env.VICTIM_ADDRESS || "0x1234...abcd";

// Option 3: Find automatically
const { findAndCheckVictims } = require("./findVictims");
const victims = await findAndCheckVictims();
const VICTIM_ADDRESS = victims[0]?.address;

// Then use in liquidation
await flashLoanContract.requestLiquidationLoanWithWETH(
    USDC_ADDR,
    debtToCover,
    VICTIM_ADDRESS
);
```

---

## ⚠️ Important Notes

1. **Health Factor Changes**: Positions can become healthy or be liquidated by others between discovery and execution
2. **Competition**: Flash loan liquidations are competitive - others may liquidate first
3. **Profitability**: Not all liquidatable positions are profitable (consider gas costs)
4. **Rate Limits**: Event scanning and subgraph queries may have rate limits
5. **Network Differences**: Addresses differ between networks (Mainnet, Arbitrum, etc.)

---

## 📊 Comparison of Methods

| Method | Speed | Accuracy | Complexity | Best For |
|--------|-------|----------|------------|----------|
| **Aave Frontend** | Slow | Medium | Easy | Manual checking |
| **Event Scanning** | Medium | High | Medium | Automated bots |
| **Subgraph** | Fast | Medium | Easy | Historical analysis |
| **Go Bot** | Fast | High | Hard | Production monitoring |
| **Manual Check** | Fast | High | Easy | Specific addresses |

---

## 🔗 Useful Links

- [Aave App](https://app.aave.com/) - Frontend to view positions
- [Aave V3 Subgraph](https://thegraph.com/hosted-service/subgraph/aave/aave-v3-ethereum) - GraphQL API
- [Aave Docs](https://docs.aave.com/) - Official documentation
- [Etherscan Aave Pool](https://etherscan.io/address/0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2) - View events directly
