const { ethers } = require("hardhat");

/**
 * Find Liquidatable Positions (Victims) on Aave
 * 
 * This script scans recent Aave events to find addresses with positions,
 * then checks their health factors to identify liquidatable positions.
 */

const AAVE_POOL = {
    mainnet: "0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2",
    arbitrum: "0x794a61358D6845594F94dc1DB02A252b5b4814aD",
    polygon: "0x794a61358D6845594F94dc1DB02A252b5b4814aD",
    optimism: "0x794a61358D6845594F94dc1DB02A252b5b4814aD"
};

async function findAndCheckVictims(blocksToScan = 5000) {
    const network = await ethers.provider.getNetwork();
    const networkName = network.name === "homestead" ? "mainnet" : network.name;
    
    const poolAddress = AAVE_POOL[networkName] || AAVE_POOL.mainnet;
    
    console.log("=== Finding Liquidatable Positions ===\n");
    console.log(`Network: ${networkName}`);
    console.log(`Aave Pool: ${poolAddress}\n`);
    
    const currentBlock = await ethers.provider.getBlockNumber();
    const fromBlock = Math.max(0, currentBlock - blocksToScan);
    
    const poolABI = [
        "event Supply(address indexed reserve, address indexed user, address indexed onBehalfOf, uint256 amount, uint16 referralCode)",
        "event Borrow(address indexed reserve, address indexed user, address indexed onBehalfOf, uint256 amount, uint8 interestRateMode, uint16 referralCode, address indexed referral)",
        "function getUserAccountData(address user) view returns (uint256 totalCollateralBase, uint256 totalDebtBase, uint256 availableBorrowsBase, uint256 currentLiquidationThreshold, uint256 ltv, uint256 healthFactor)"
    ];
    
    const pool = await ethers.getContractAt(poolABI, poolAddress);
    
    console.log(`Scanning blocks ${fromBlock} to ${currentBlock}...`);
    
    // Get events
    const supplyFilter = pool.filters.Supply();
    const borrowFilter = pool.filters.Borrow();
    
    console.log("Fetching Supply and Borrow events...");
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
    
    console.log(`Found ${addresses.size} unique addresses with Aave activity\n`);
    console.log("Checking health factors...\n");
    
    // Check each address
    const victims = [];
    let checked = 0;
    const addressesArray = Array.from(addresses);
    
    for (const addr of addressesArray) {
        checked++;
        if (checked % 50 === 0) {
            console.log(`Checked ${checked}/${addressesArray.length} addresses...`);
        }
        
        try {
            const userData = await pool.getUserAccountData(addr);
            const hf = Number(ethers.formatUnits(userData.healthFactor, 27));
            
            // Health factor < 1.0 means liquidatable
            // Health factor > 0 means they have a position
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
                    debt: userData.totalDebtBase,
                    collateralFormatted: collateral,
                    debtFormatted: debt
                });
            }
        } catch (error) {
            // Skip errors (no position, contract errors, etc.)
        }
    }
    
    console.log(`\n=== Summary ===`);
    console.log(`Total addresses checked: ${checked}`);
    console.log(`Liquidatable positions found: ${victims.length}`);
    
    if (victims.length > 0) {
        console.log(`\n📋 Victim Addresses (Health Factor < 1.0):`);
        console.log("─".repeat(80));
        victims.forEach((v, i) => {
            console.log(`${i + 1}. ${v.address}`);
            console.log(`   Health Factor: ${v.healthFactor.toFixed(4)}`);
            console.log(`   Collateral: ${v.collateralFormatted}`);
            console.log(`   Debt: ${v.debtFormatted}`);
            console.log("");
        });
        console.log("─".repeat(80));
        console.log(`\n💡 To use a victim address in your liquidation script:`);
        console.log(`   VICTIM_ADDRESS=${victims[0].address} npx hardhat run scripts/flashloanLiquidation.js --network ${networkName}`);
    } else {
        console.log("\n⚠️  No liquidatable positions found in the scanned blocks.");
        console.log("   Try:");
        console.log("   - Scanning more blocks (increase blocksToScan parameter)");
        console.log("   - Checking during volatile market conditions");
        console.log("   - Using the Aave frontend: https://app.aave.com/");
    }
    
    return victims;
}

// Main execution
async function main() {
    const blocksToScan = process.env.BLOCKS_TO_SCAN ? parseInt(process.env.BLOCKS_TO_SCAN) : 5000;
    
    try {
        const victims = await findAndCheckVictims(blocksToScan);
        process.exit(0);
    } catch (error) {
        console.error("\n❌ Error finding victims:", error.message);
        if (error.message.includes("network")) {
            console.error("\n💡 Tip: Make sure you're connected to the correct network");
            console.error("   For mainnet: --network mainnet");
            console.error("   For hardhat fork: --network hardhat");
        }
        process.exit(1);
    }
}

// Export for use in other scripts
module.exports = { findAndCheckVictims };

// Run if called directly
if (require.main === module) {
    main();
}
