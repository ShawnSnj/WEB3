const { ethers } = require("hardhat");

/**
 * Deploy FlashLoanExample Contract
 * 
 * This script deploys the flash loan contract to the network.
 * Make sure to update the POOL_ADDRESSES_PROVIDER for your target network.
 */

async function main() {
    const [deployer] = await ethers.getSigners();
    console.log("=== Deploying FlashLoanExample Contract ===\n");
    console.log("Deployer address:", deployer.address);
    
    const balance = await ethers.provider.getBalance(deployer.address);
    console.log("Deployer balance:", ethers.formatEther(balance), "ETH\n");

    // Network-specific Pool Addresses Provider
    // Ethereum Mainnet
    const poolAddressesProvider = "0x2f39d218133AFaB8F2B819B1066c7E434Ad94E9e";
    
    // For other networks, update accordingly:
    // Arbitrum: 0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb
    // Polygon: 0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb
    // Avalanche: 0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb

    console.log("Pool Addresses Provider:", poolAddressesProvider);
    console.log("Deploying contract...\n");

    const FlashLoan = await ethers.getContractFactory("FlashLoanExample");
    const flashLoan = await FlashLoan.deploy(poolAddressesProvider);
    await flashLoan.waitForDeployment();

    const contractAddress = await flashLoan.getAddress();
    console.log("✓ FlashLoanExample deployed successfully!");
    console.log("Contract address:", contractAddress);
    console.log("\n=== Deployment Complete ===");
    console.log("\nNext steps:");
    console.log("1. Save this address for future use");
    console.log("2. Fund the contract with tokens to cover flash loan premium");
    console.log("3. Run flashloanLiquidation.js to test\n");
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    });