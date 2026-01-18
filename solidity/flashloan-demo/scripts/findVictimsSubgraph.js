const axios = require("axios");

/**
 * Find Liquidatable Positions using Aave Subgraph
 * 
 * This script queries the Aave subgraph to find positions with Health Factor < 1.0
 * Much faster than event scanning, but may be 1-5 minutes behind on-chain state.
 */

const SUBGRAPHS = {
    mainnet: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-ethereum",
    arbitrum: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-arbitrum",
    polygon: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-polygon",
    optimism: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-optimism",
    avalanche: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-avalanche",
    base: "https://api.thegraph.com/subgraphs/name/aave/aave-v3-base"
};

/**
 * Query the subgraph with a GraphQL query
 */
async function querySubgraph(subgraphUrl, query) {
    try {
        const response = await axios.post(subgraphUrl, { query }, {
            timeout: 10000, // 10 second timeout
            headers: {
                'Content-Type': 'application/json'
            }
        });

        if (response.data.errors) {
            throw new Error(`GraphQL errors: ${JSON.stringify(response.data.errors)}`);
        }

        return response.data.data;
    } catch (error) {
        if (error.response) {
            throw new Error(`Subgraph query failed: ${error.response.status} - ${JSON.stringify(error.response.data)}`);
        }
        throw new Error(`Subgraph query error: ${error.message}`);
    }
}

/**
 * Find liquidatable positions from subgraph
 */
async function findLiquidatablePositions(network = "mainnet", options = {}) {
    const {
        limit = 100,
        minDebtUSD = 0,
        maxHealthFactor = 1.0,
        minHealthFactor = 0
    } = options;

    const subgraphUrl = SUBGRAPHS[network];
    if (!subgraphUrl) {
        throw new Error(`Unknown network: ${network}. Available: ${Object.keys(SUBGRAPHS).join(", ")}`);
    }

    console.log(`\n=== Finding Liquidatable Positions on ${network.toUpperCase()} ===\n`);
    console.log(`Subgraph URL: ${subgraphUrl}`);
    console.log(`Filters:`);
    console.log(`  - Health Factor: ${minHealthFactor} < HF < ${maxHealthFactor}`);
    console.log(`  - Min Debt: $${minDebtUSD}`);
    console.log(`  - Limit: ${limit}\n`);

    // Build where clause
    const whereConditions = [
        `healthFactor_lt: "${maxHealthFactor}"`,
        `healthFactor_gt: "${minHealthFactor}"`
    ];

    if (minDebtUSD > 0) {
        whereConditions.push(`totalDebtUSD_gt: "${minDebtUSD}"`);
    }

    const whereClause = whereConditions.join(", ");

    const query = `
        {
            userAccounts(
                where: {
                    ${whereClause}
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
        console.log("Querying subgraph...");
        const data = await querySubgraph(subgraphUrl, query);
        const victims = data.userAccounts || [];

        if (victims.length === 0) {
            console.log("⚠️  No liquidatable positions found with current filters.\n");
            console.log("💡 Try:");
            console.log("   - Increasing the limit");
            console.log("   - Lowering minDebtUSD");
            console.log("   - Checking a different network");
            return [];
        }

        console.log(`✓ Found ${victims.length} liquidatable positions:\n`);
        console.log("─".repeat(100));

        victims.forEach((v, i) => {
            const hf = parseFloat(v.healthFactor);
            const collateral = parseFloat(v.totalCollateralUSD);
            const debt = parseFloat(v.totalDebtUSD);
            const riskLevel = hf < 0.5 ? "🔴 CRITICAL" : hf < 0.8 ? "🟠 HIGH" : "🟡 MEDIUM";

            console.log(`${i + 1}. ${v.id}`);
            console.log(`   Health Factor: ${hf.toFixed(4)} ${riskLevel}`);
            console.log(`   Collateral: $${collateral.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`);
            console.log(`   Debt: $${debt.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`);
            console.log("");
        });

        console.log("─".repeat(100));
        console.log(`\n📊 Summary:`);
        console.log(`   Total positions: ${victims.length}`);
        console.log(`   Lowest HF: ${parseFloat(victims[0].healthFactor).toFixed(4)}`);
        console.log(`   Highest debt: $${Math.max(...victims.map(v => parseFloat(v.totalDebtUSD))).toLocaleString()}`);

        return victims;
    } catch (error) {
        console.error("\n❌ Error querying subgraph:", error.message);
        throw error;
    }
}

/**
 * Get all liquidatable positions with pagination
 */
async function getAllLiquidatablePositions(network = "mainnet", options = {}) {
    const {
        minDebtUSD = 0,
        maxHealthFactor = 1.0,
        minHealthFactor = 0,
        maxResults = 1000
    } = options;

    const subgraphUrl = SUBGRAPHS[network];
    const allVictims = [];
    let skip = 0;
    const pageSize = 100;
    let hasMore = true;

    console.log(`\n=== Fetching ALL Liquidatable Positions (with pagination) ===\n`);

    while (hasMore && allVictims.length < maxResults) {
        const whereConditions = [
            `healthFactor_lt: "${maxHealthFactor}"`,
            `healthFactor_gt: "${minHealthFactor}"`
        ];

        if (minDebtUSD > 0) {
            whereConditions.push(`totalDebtUSD_gt: "${minDebtUSD}"`);
        }

        const whereClause = whereConditions.join(", ");

        const query = `
            {
                userAccounts(
                    where: {
                        ${whereClause}
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

        try {
            const data = await querySubgraph(subgraphUrl, query);
            const victims = data.userAccounts || [];

            if (victims.length === 0) {
                hasMore = false;
            } else {
                allVictims.push(...victims);
                skip += pageSize;
                console.log(`Fetched ${allVictims.length} positions so far...`);
                
                // Small delay to avoid rate limits
                await new Promise(resolve => setTimeout(resolve, 200));
            }
        } catch (error) {
            console.error(`Error fetching page (skip=${skip}):`, error.message);
            hasMore = false;
        }
    }

    console.log(`\n✓ Total positions found: ${allVictims.length}\n`);
    return allVictims;
}

/**
 * Get details for a specific user
 */
async function getUserPosition(network, userAddress) {
    const subgraphUrl = SUBGRAPHS[network];
    if (!subgraphUrl) {
        throw new Error(`Unknown network: ${network}`);
    }

    const query = `
        {
            userAccount(id: "${userAddress.toLowerCase()}") {
                id
                healthFactor
                totalCollateralUSD
                totalDebtUSD
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
    `;

    try {
        const data = await querySubgraph(subgraphUrl, query);
        return data.userAccount;
    } catch (error) {
        console.error(`Error fetching user position:`, error.message);
        throw error;
    }
}

// Main execution
async function main() {
    const args = process.argv.slice(2);
    const network = args[0] || process.env.NETWORK || "mainnet";
    const mode = args[1] || "find"; // "find" or "all" or "user"

    try {
        if (mode === "all") {
            // Get all positions with pagination
            const victims = await getAllLiquidatablePositions(network, {
                minDebtUSD: 1000, // Only positions with > $1k debt
                maxHealthFactor: 1.0
            });
            
            console.log(`\n💡 To use a victim address:`);
            if (victims.length > 0) {
                console.log(`   VICTIM_ADDRESS=${victims[0].id} npx hardhat run scripts/flashloanLiquidation.js --network ${network}`);
            }
        } else if (mode === "user" && args[2]) {
            // Get specific user
            const userAddress = args[2];
            console.log(`\n=== User Position Details ===\n`);
            const position = await getUserPosition(network, userAddress);
            
            if (!position) {
                console.log(`⚠️  No position found for ${userAddress}`);
                return;
            }

            console.log(`Address: ${position.id}`);
            console.log(`Health Factor: ${position.healthFactor}`);
            console.log(`Total Collateral: $${parseFloat(position.totalCollateralUSD).toFixed(2)}`);
            console.log(`Total Debt: $${parseFloat(position.totalDebtUSD).toFixed(2)}`);
            console.log(`\nReserves:`);
            position.reserves.forEach(r => {
                if (parseFloat(r.currentATokenBalance) > 0 || 
                    parseFloat(r.currentStableDebt) > 0 || 
                    parseFloat(r.currentVariableDebt) > 0) {
                    console.log(`  ${r.reserve.symbol}:`);
                    if (parseFloat(r.currentATokenBalance) > 0) {
                        console.log(`    Collateral: ${parseFloat(r.currentATokenBalance).toFixed(4)}`);
                    }
                    if (parseFloat(r.currentStableDebt) > 0) {
                        console.log(`    Stable Debt: ${parseFloat(r.currentStableDebt).toFixed(4)}`);
                    }
                    if (parseFloat(r.currentVariableDebt) > 0) {
                        console.log(`    Variable Debt: ${parseFloat(r.currentVariableDebt).toFixed(4)}`);
                    }
                }
            });
        } else {
            // Default: find liquidatable positions
            const victims = await findLiquidatablePositions(network, {
                limit: 50,
                minDebtUSD: 1000, // Only positions with > $1k debt
                maxHealthFactor: 1.0
            });

            if (victims.length > 0) {
                console.log(`\n💡 To use a victim address in your liquidation script:`);
                console.log(`   VICTIM_ADDRESS=${victims[0].id} npx hardhat run scripts/flashloanLiquidation.js --network ${network}`);
                console.log(`\n⚠️  Remember: Subgraph may be 1-5 minutes behind. Verify on-chain before liquidating!`);
            }
        }

        process.exit(0);
    } catch (error) {
        console.error("\n❌ Error:", error.message);
        process.exit(1);
    }
}

// Export functions for use in other scripts
module.exports = {
    findLiquidatablePositions,
    getAllLiquidatablePositions,
    getUserPosition,
    SUBGRAPHS
};

// Run if called directly
if (require.main === module) {
    main();
}
