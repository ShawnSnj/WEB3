const { ethers, network } = require("hardhat");
const { getNetworkConfig } = require("../config/networks");

/**
 * Production Deployment Script
 * 
 * Deploys FlashLoanLiquidation contract with proper configuration for production networks
 * 
 * Usage:
 *   npx hardhat run scripts/deployProduction.js --network mainnet
 *   npx hardhat run scripts/deployProduction.js --network arbitrum
 */

async function main() {
    console.log("=== Production Deployment ===\n");
    
    const networkName = network.name;
    console.log(`Network: ${networkName}`);
    
    if (networkName === "hardhat") {
        console.log("⚠️  This script is for production networks only.");
        console.log("   Use scripts/deploy.js for local testing.\n");
        return;
    }
    
    // Get network configuration
    const config = getNetworkConfig(networkName);
    console.log(`Chain: ${config.name} (Chain ID: ${config.chainId})`);
    console.log(`Explorer: ${config.explorer}\n`);
    
    // Get deployer
    const [deployer] = await ethers.getSigners();
    console.log("Deployer:", deployer.address);
    
    const balance = await ethers.provider.getBalance(deployer.address);
    console.log("Balance:", ethers.formatEther(balance), "ETH");
    
    if (balance < ethers.parseEther("0.01")) {
        console.log("\n⚠️  WARNING: Low balance! Ensure you have enough ETH for gas fees.\n");
    }
    
    // Deployment parameters
    let addressProvider = config.aavePoolAddressesProvider;
    const swapRouter = config.useV3 ? config.uniswapV3Router : config.uniswapV2Router;
    const useV3 = config.useV3;
    const defaultPoolFee = config.defaultPoolFee;
    const maxSlippageBps = 100; // 1% default slippage
    
    // Validate and fix address if needed
    if (!ethers.isAddress(addressProvider)) {
        console.log("⚠️  Invalid PoolAddressesProvider address, attempting to derive from Pool...");
        try {
            // Try to get PoolAddressesProvider from Pool contract
            const poolABI = [
                "function ADDRESSES_PROVIDER() view returns (address)"
            ];
            const pool = await ethers.getContractAt(poolABI, config.aavePool);
            addressProvider = await pool.ADDRESSES_PROVIDER();
            console.log("   ✓ Derived from Pool:", addressProvider);
        } catch (error) {
            throw new Error(
                `Invalid PoolAddressesProvider address: ${config.aavePoolAddressesProvider}\n` +
                `Could not derive from Pool. Please verify Aave V3 is deployed on ${networkName}.\n` +
                `Consider using a different testnet (Goerli, Mumbai) or mainnet.`
            );
        }
    }
    
    // Validate swap router
    if (!ethers.isAddress(swapRouter)) {
        throw new Error(`Invalid SwapRouter address: ${swapRouter}`);
    }
    
    console.log("\n=== Deployment Configuration ===");
    console.log("Aave PoolAddressesProvider:", addressProvider);
    console.log("Swap Router:", swapRouter);
    console.log("Using Uniswap V3:", useV3);
    if (useV3) {
        console.log("Default Pool Fee:", defaultPoolFee);
    }
    console.log("Max Slippage:", maxSlippageBps, "bps (1%)\n");
    
    // Deploy contract
    console.log("Deploying FlashLoanLiquidation contract...");
    const FlashLoanLiquidation = await ethers.getContractFactory("FlashLoanLiquidation");
    
    // Get checksummed addresses
    const addressProviderAddr = ethers.getAddress(addressProvider);
    const swapRouterAddr = ethers.getAddress(swapRouter);
    
    console.log("Validated addresses:");
    console.log("  PoolAddressesProvider:", addressProviderAddr);
    console.log("  SwapRouter:", swapRouterAddr);
    
    const contract = await FlashLoanLiquidation.deploy(
        addressProviderAddr,
        swapRouterAddr,
        useV3,
        defaultPoolFee,
        maxSlippageBps
    );
    
    console.log("Transaction sent, waiting for confirmation...");
    await contract.waitForDeployment();
    
    const contractAddress = await contract.getAddress();
    console.log("\n✓ Contract deployed!");
    console.log("Address:", contractAddress);
    console.log(`Explorer: ${config.explorer}/address/${contractAddress}\n`);
    
    // Verify deployment
    console.log("=== Verifying Deployment ===");
    try {
        const owner = await contract.owner();
        const swapRouterAddr = await contract.swapRouter();
        const useV3Router = await contract.useV3Router();
        
        console.log("Owner:", owner);
        console.log("Swap Router:", swapRouterAddr);
        console.log("Using V3 Router:", useV3Router);
        
        if (owner.toLowerCase() !== deployer.address.toLowerCase()) {
            console.log("⚠️  WARNING: Owner mismatch!");
        }
        
        if (swapRouterAddr.toLowerCase() !== swapRouter.toLowerCase()) {
            console.log("⚠️  WARNING: Swap router mismatch!");
        }
        
        console.log("\n✓ Deployment verified!\n");
    } catch (error) {
        console.log("⚠️  Could not verify deployment:", error.message);
    }
    
    // Save deployment info
    const deploymentInfo = {
        network: networkName,
        chainId: config.chainId,
        contractAddress: contractAddress,
        deployer: deployer.address,
        blockNumber: await ethers.provider.getBlockNumber(),
        timestamp: new Date().toISOString(),
        config: {
            addressProvider,
            swapRouter,
            useV3,
            defaultPoolFee,
            maxSlippageBps
        }
    };
    
    const fs = require("fs");
    const path = require("path");
    const deploymentsDir = path.join(__dirname, "..", "deployments");
    
    if (!fs.existsSync(deploymentsDir)) {
        fs.mkdirSync(deploymentsDir, { recursive: true });
    }
    
    const deploymentFile = path.join(deploymentsDir, `${networkName}.json`);
    fs.writeFileSync(deploymentFile, JSON.stringify(deploymentInfo, null, 2));
    
    console.log("=== Deployment Summary ===");
    console.log(`Contract Address: ${contractAddress}`);
    console.log(`Network: ${config.name}`);
    console.log(`Deployment saved to: ${deploymentFile}`);
    console.log(`\nNext steps:`);
    console.log(`1. Verify contract on ${config.explorer}`);
    console.log(`2. Fund the contract if needed (for edge cases)`);
    console.log(`3. Update your bot/config with the contract address`);
    console.log(`4. Test with a small liquidation first!\n`);
    
    // Verification command
    if (process.env.ETHERSCAN_API_KEY || process.env.ARBISCAN_API_KEY) {
        console.log("To verify on explorer, run:");
        console.log(`npx hardhat verify --network ${networkName} ${contractAddress} ${addressProvider} ${swapRouter} ${useV3} ${defaultPoolFee} ${maxSlippageBps}\n`);
    }
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error("\n❌ Deployment failed:", error);
        process.exit(1);
    });
