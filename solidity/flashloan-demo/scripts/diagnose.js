const { ethers, network } = require("hardhat");
const { getNetworkConfig } = require("../config/networks");

/**
 * Diagnostic Script
 * 
 * Checks common deployment issues and configuration problems
 * 
 * Usage:
 *   npx hardhat run scripts/diagnose.js --network sepolia
 */

async function main() {
    console.log("=== Deployment Diagnostics ===\n");
    
    const networkName = network.name;
    console.log(`Network: ${networkName}\n`);
    
    // Check 1: Network Configuration
    console.log("1. Checking network configuration...");
    try {
        const config = getNetworkConfig(networkName);
        console.log("   ✓ Network config found");
        console.log(`   Chain: ${config.name} (${config.chainId})`);
        console.log(`   RPC: ${config.rpcUrl}`);
        console.log(`   Explorer: ${config.explorer}`);
    } catch (error) {
        console.log("   ✗ Network config error:", error.message);
        console.log("   Available networks:", require("../config/networks").getSupportedNetworks().join(", "));
        return;
    }
    
    // Check 2: RPC Connection
    console.log("\n2. Checking RPC connection...");
    try {
        const provider = ethers.provider;
        const blockNumber = await provider.getBlockNumber();
        console.log("   ✓ RPC connected");
        console.log(`   Current block: ${blockNumber}`);
    } catch (error) {
        console.log("   ✗ RPC connection failed:", error.message);
        console.log("   Check your RPC_URL in .env");
        return;
    }
    
    // Check 3: Wallet Configuration
    console.log("\n3. Checking wallet configuration...");
    try {
        const [signer] = await ethers.getSigners();
        console.log("   ✓ Wallet configured");
        console.log(`   Address: ${signer.address}`);
        
        const balance = await ethers.provider.getBalance(signer.address);
        const balanceEth = ethers.formatEther(balance);
        console.log(`   Balance: ${balanceEth} ETH`);
        
        if (balance < ethers.parseEther("0.01")) {
            console.log("   ⚠️  WARNING: Low balance! May not have enough for gas");
        } else {
            console.log("   ✓ Sufficient balance for deployment");
        }
    } catch (error) {
        console.log("   ✗ Wallet error:", error.message);
        console.log("   Check PRIVATE_KEY in .env");
        return;
    }
    
    // Check 4: Contract Compilation
    console.log("\n4. Checking contract compilation...");
    try {
        await hre.run("compile", { quiet: true });
        console.log("   ✓ Contracts compiled successfully");
    } catch (error) {
        console.log("   ✗ Compilation failed:", error.message);
        console.log("   Run: npx hardhat compile");
        return;
    }
    
    // Check 5: Network Addresses
    console.log("\n5. Checking network addresses...");
    try {
        const config = getNetworkConfig(networkName);
        console.log("   Aave PoolAddressesProvider:", config.aavePoolAddressesProvider);
        console.log("   Aave Pool:", config.aavePool);
        console.log("   Swap Router:", config.useV3 ? config.uniswapV3Router : config.uniswapV2Router);
        
        // Verify addresses are valid
        if (!ethers.isAddress(config.aavePoolAddressesProvider)) {
            console.log("   ⚠️  WARNING: Invalid PoolAddressesProvider address");
        }
        if (!ethers.isAddress(config.aavePool)) {
            console.log("   ⚠️  WARNING: Invalid Pool address");
        }
    } catch (error) {
        console.log("   ✗ Address check failed:", error.message);
    }
    
    // Check 6: Aave Pool Connection
    console.log("\n6. Checking Aave Pool connection...");
    try {
        const config = getNetworkConfig(networkName);
        const poolABI = [
            "function getReserveData(address asset) view returns (tuple(uint256 configuration, uint128 liquidityIndex, uint128 currentLiquidityRate, uint128 variableBorrowIndex, uint128 currentVariableBorrowRate, uint128 currentStableBorrowRate, uint40 lastUpdateTimestamp, uint16 id, address aTokenAddress, address stableDebtTokenAddress, address variableDebtTokenAddress, address interestRateStrategyAddress, uint128 accruedToTreasury, uint128 unbacked, uint128 isolationModeTotalDebt))"
        ];
        const pool = await ethers.getContractAt(poolABI, config.aavePool);
        
        // Try to get reserve data for WETH
        const wethAddress = config.tokens.WETH;
        if (wethAddress) {
            const reserveData = await pool.getReserveData(wethAddress);
            console.log("   ✓ Aave Pool accessible");
            console.log("   ✓ Can read reserve data");
        }
    } catch (error) {
        console.log("   ✗ Aave Pool connection failed:", error.message);
        console.log("   Check Pool address is correct for network");
    }
    
    // Check 7: Deployment Readiness
    console.log("\n7. Deployment readiness check...");
    const config = getNetworkConfig(networkName);
    const issues = [];
    
    if (!process.env.PRIVATE_KEY) {
        issues.push("PRIVATE_KEY not set in .env");
    }
    
    const [signer] = await ethers.getSigners();
    const balance = await ethers.provider.getBalance(signer.address);
    if (balance < ethers.parseEther("0.01")) {
        issues.push("Low balance (need at least 0.01 ETH)");
    }
    
    if (issues.length === 0) {
        console.log("   ✓ Ready to deploy!");
        console.log("\n   Next step:");
        console.log(`   npx hardhat run scripts/deployProduction.js --network ${networkName}`);
    } else {
        console.log("   ⚠️  Issues found:");
        issues.forEach(issue => console.log(`   - ${issue}`));
    }
    
    console.log("\n=== Diagnostics Complete ===");
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error("\n❌ Diagnostic failed:", error);
        process.exit(1);
    });
