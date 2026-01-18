# Aave Subgraph Guide

A **subgraph** is an indexed database of blockchain data that makes querying much faster than scanning events directly. Aave maintains public subgraphs for all their deployments.

---

## 🎯 What is a Subgraph?

Instead of scanning thousands of blocks for events, you query a pre-indexed GraphQL API that has all the data ready.

**Advantages:**
- ✅ **Fast**: Pre-indexed data, instant queries
- ✅ **Historical**: Access all historical data easily
- ✅ **Rich Data**: Includes calculated fields like health factors
- ✅ **No RPC Limits**: Doesn't count against your RPC rate limits

**Disadvantages:**
- ⚠️ **Delay**: Usually 1-5 minutes behind on-chain state
- ⚠️ **May Miss Recent**: Very recent positions might not be indexed yet

---

## 📡 Aave Subgraph URLs

### Ethereum Mainnet
```
https://api.thegraph.com/subgraphs/name/aave/aave-v3-ethereum
```

### Arbitrum
```
https://api.thegraph.com/subgraphs/name/aave/aave-v3-arbitrum
```

### Polygon
```
https://api.thegraph.com/subgraphs/name/aave/aave-v3-polygon
```

### Optimism
```
https://api.thegraph.com/subgraphs/name/aave/aave-v3-optimism
```

### Avalanche
```
https://api.thegraph.com/subgraphs/name/aave/aave-v3-avalanche
```

### Base
```
https://api.thegraph.com/subgraphs/name/aave/aave-v3-base
```

---

## 🔍 GraphQL Basics

Subgraphs use **GraphQL**, a query language. Here's the basic structure:

```graphql
{
  entityName(
    where: { field: value }    # Filter conditions
    first: 100                  # Limit results
    orderBy: field             # Sort by
    orderDirection: desc        # asc or desc
  ) {
    field1                      # Fields to return
    field2
    nestedEntity {              # Nested queries
      nestedField
    }
  }
}
```

---

## 📋 Common Queries

### 1. Find Liquidatable Positions (Health Factor < 1.0)

```graphql
{
  userAccounts(
    where: {
      healthFactor_lt: "1.0"
      healthFactor_gt: "0"
    }
    first: 100
    orderBy: healthFactor
    orderDirection: asc
  ) {
    id
    healthFactor
    totalCollateralUSD
    totalDebtUSD
  }
}
```

**What it returns:**
- `id`: User address (victim address)
- `healthFactor`: Health factor as string (e.g., "0.95")
- `totalCollateralUSD`: Total collateral in USD
- `totalDebtUSD`: Total debt in USD

### 2. Find Positions with High Debt

```graphql
{
  userAccounts(
    where: {
      totalDebtUSD_gt: "10000"
      healthFactor_lt: "1.5"
    }
    first: 50
    orderBy: totalDebtUSD
    orderDirection: desc
  ) {
    id
    healthFactor
    totalCollateralUSD
    totalDebtUSD
  }
}
```

### 3. Get Specific User's Position

```graphql
{
  userAccount(id: "0x1234...abcd") {
    id
    healthFactor
    totalCollateralUSD
    totalDebtUSD
    reserves {
      currentATokenBalance
      currentStableDebt
      currentVariableDebt
      reserve {
        symbol
        decimals
      }
    }
  }
}
```

### 4. Find Recent Liquidations

```graphql
{
  liquidationCalls(
    first: 10
    orderBy: timestamp
    orderDirection: desc
  ) {
    id
    user {
      id
    }
    collateralReserve {
      symbol
    }
    principalReserve {
      symbol
    }
    collateralAmount
    principalAmount
    timestamp
  }
}
```

---

## 💻 JavaScript Examples

### Basic Query Function

```javascript
const axios = require("axios");

async function querySubgraph(subgraphUrl, query) {
    try {
        const response = await axios.post(subgraphUrl, { query });
        return response.data.data;
    } catch (error) {
        console.error("Subgraph query error:", error.message);
        if (error.response) {
            console.error("Response:", error.response.data);
        }
        throw error;
    }
}

// Usage
const subgraphUrl = "https://api.thegraph.com/subgraphs/name/aave/aave-v3-ethereum";

const query = `
    {
        userAccounts(
            where: {
                healthFactor_lt: "1.0"
                healthFactor_gt: "0"
            }
            first: 10
        ) {
            id
            healthFactor
            totalCollateralUSD
            totalDebtUSD
        }
    }
`;

const data = await querySubgraph(subgraphUrl, query);
console.log(data.userAccounts);
```

### Find Victims Script

```javascript
const axios = require("axios");

const SUBGRAPHS = {
    mainnet: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-ethereum",
    arbitrum: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-arbitrum",
    polygon: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-polygon",
    optimism: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-optimism",
    avalanche: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-avalanche",
    base: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-base"
};

async function findLiquidatablePositions(network = "mainnet", limit = 100) {
    const subgraphUrl = SUBGRAPHS[network];
    if (!subgraphUrl) {
        throw new Error(`Unknown network: ${network}`);
    }

    const query = `
        {
            userAccounts(
                where: {
                    healthFactor_lt: "1.0"
                    healthFactor_gt: "0"
                }
                first: ${limit}
                orderBy: healthFactor
                orderDirection: asc
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

        console.log(`Found ${victims.length} liquidatable positions on ${network}:\n`);
        
        victims.forEach((v, i) => {
            console.log(`${i + 1}. ${v.id}`);
            console.log(`   Health Factor: ${v.healthFactor}`);
            console.log(`   Collateral: $${parseFloat(v.totalCollateralUSD).toFixed(2)}`);
            console.log(`   Debt: $${parseFloat(v.totalDebtUSD).toFixed(2)}`);
            console.log("");
        });

        return victims;
    } catch (error) {
        console.error("Error querying subgraph:", error.message);
        throw error;
    }
}

// Run
findLiquidatablePositions("mainnet", 20)
    .then(victims => {
        console.log(`\nTotal: ${victims.length} liquidatable positions`);
    })
    .catch(console.error);
```

---

## 🌐 Using in Browser (Frontend)

You can also query subgraphs directly from a browser:

```javascript
async function querySubgraph(query) {
    const subgraphUrl = "https://api.thegraph.com/subgraphs/name/aave/aave-v3-ethereum";
    
    const response = await fetch(subgraphUrl, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify({ query }),
    });
    
    const data = await response.json();
    return data.data;
}

// Usage
const query = `
    {
        userAccounts(
            where: { healthFactor_lt: "1.0", healthFactor_gt: "0" }
            first: 10
        ) {
            id
            healthFactor
            totalCollateralUSD
            totalDebtUSD
        }
    }
`;

const data = await querySubgraph(query);
console.log(data.userAccounts);
```

---

## 🔧 Advanced Queries

### Pagination (Get More Results)

Subgraphs limit results (usually 100-1000 per query). Use pagination:

```javascript
async function getAllLiquidatablePositions(network = "mainnet") {
    const subgraphUrl = SUBGRAPHS[network];
    const allVictims = [];
    let skip = 0;
    const pageSize = 100;
    let hasMore = true;

    while (hasMore) {
        const query = `
            {
                userAccounts(
                    where: {
                        healthFactor_lt: "1.0"
                        healthFactor_gt: "0"
                    }
                    first: ${pageSize}
                    skip: ${skip}
                    orderBy: healthFactor
                    orderDirection: asc
                ) {
                    id
                    healthFactor
                    totalCollateralUSD
                    totalDebtUSD
                }
            }
        `;

        const response = await axios.post(subgraphUrl, { query });
        const victims = response.data.data.userAccounts;

        if (victims.length === 0) {
            hasMore = false;
        } else {
            allVictims.push(...victims);
            skip += pageSize;
            console.log(`Fetched ${allVictims.length} positions so far...`);
        }
    }

    return allVictims;
}
```

### Filter by Minimum Debt

```graphql
{
  userAccounts(
    where: {
      healthFactor_lt: "1.0"
      healthFactor_gt: "0"
      totalDebtUSD_gt: "5000"    # Only positions with > $5k debt
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

### Get User's Reserves (Collateral & Debt Details)

```graphql
{
  userAccount(id: "0x1234...abcd") {
    id
    healthFactor
    reserves {
      currentATokenBalance
      currentStableDebt
      currentVariableDebt
      reserve {
        id
        symbol
        name
        decimals
        underlyingAsset
      }
    }
  }
}
```

---

## 🧪 Testing Queries

### Option 1: GraphQL Playground

Visit The Graph's hosted service:
1. Go to https://thegraph.com/hosted-service/
2. Search for "aave-v3-ethereum" (or your network)
3. Click "Query" button
4. Use the GraphQL playground to test queries

### Option 2: curl (Command Line)

```bash
curl -X POST \
  https://api.thegraph.com/subgraphs/name/aave/aave-v3-ethereum \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "{ userAccounts(where: { healthFactor_lt: \"1.0\", healthFactor_gt: \"0\" }, first: 5) { id healthFactor totalCollateralUSD totalDebtUSD } }"
  }'
```

### Option 3: Postman / Insomnia

1. Method: POST
2. URL: `https://api.thegraph.com/subgraphs/name/aave/aave-v3-ethereum`
3. Headers: `Content-Type: application/json`
4. Body (JSON):
```json
{
  "query": "{ userAccounts(where: { healthFactor_lt: \"1.0\" }, first: 5) { id healthFactor } }"
}
```

---

## ⚠️ Important Notes

### 1. Health Factor Format
Health factors in subgraph are **strings**, not numbers:
- `"1.5"` = healthy
- `"0.95"` = liquidatable
- `"0"` = no position

### 2. USD Values
All USD values are strings representing decimal numbers:
- `"10000.50"` = $10,000.50

### 3. Delay
Subgraph is usually **1-5 minutes** behind on-chain state:
- A position might be liquidated on-chain but still show in subgraph
- Always verify on-chain before executing liquidation

### 4. Rate Limits
The Graph's hosted service has rate limits:
- Free tier: ~100 requests/minute
- If you hit limits, consider:
  - Adding delays between requests
  - Using your own subgraph indexer
  - Using event scanning as backup

### 5. Verify On-Chain
Before liquidating, always verify the position on-chain:

```javascript
const { ethers } = require("hardhat");

async function verifyPositionOnChain(address) {
    const AAVE_POOL = "0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2";
    const poolABI = [
        "function getUserAccountData(address user) view returns (uint256 totalCollateralBase, uint256 totalDebtBase, uint256 availableBorrowsBase, uint256 currentLiquidationThreshold, uint256 ltv, uint256 healthFactor)"
    ];
    
    const pool = await ethers.getContractAt(poolABI, AAVE_POOL);
    const userData = await pool.getUserAccountData(address);
    const hf = Number(ethers.formatUnits(userData.healthFactor, 27));
    
    return hf < 1.0; // Still liquidatable?
}
```

---

## 📊 Complete Working Script

See `scripts/findVictimsSubgraph.js` for a complete, production-ready script that:
- Queries multiple networks
- Handles pagination
- Verifies positions on-chain
- Formats output nicely
- Can be integrated with your liquidation bot

---

## 🔗 Useful Links

- **The Graph Explorer**: https://thegraph.com/hosted-service/
- **Aave V3 Ethereum Subgraph**: https://thegraph.com/hosted-service/subgraph/aave/aave-v3-ethereum
- **GraphQL Documentation**: https://graphql.org/learn/
- **Aave Docs**: https://docs.aave.com/

---

## 🎯 Quick Start

1. **Install axios** (if not already installed):
   ```bash
   npm install axios
   ```

2. **Run the script**:
   ```bash
   node scripts/findVictimsSubgraph.js
   ```

3. **Use in your liquidation script**:
   ```javascript
   const { findLiquidatablePositions } = require("./findVictimsSubgraph");
   const victims = await findLiquidatablePositions("mainnet");
   const victimAddress = victims[0].id;
   ```

That's it! The subgraph makes finding liquidatable positions much easier than scanning events.
