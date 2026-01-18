const { ethers, network } = require("hardhat");
const { getNetworkConfig } = require("../config/networks");
const fs = require("fs");
const path = require("path");

/**
 * Production Liquidation Script
 * 
 * Executes a flash loan liquidation on a real network
 * 
 * Usage:
 *   VICTIM_ADDRESS=0x... npx hardhat run scripts/liquidateProduction.js --network mainnet
 *   VICTIM_ADDRESS=0x... DEBT_TOKEN=0x... COLLATERAL_TOKEN=0x... DEBT_AMOUNT=5000000000 npx hardhat run scripts/liquidateProduction.js --network arbitrum
 */

async function main() {
    console.log("=== Production Flash Loan Liquidation ===\n");
    
    const networkName = network.name;
    
    if (networkName === "hardhat") {
        console.log("⚠️  This script is for production networks only.");
        console.log("   Use scripts/flashloanLiquidation.js for local testing.\n");
        return;
    }
    
    const config = getNetworkConfig(networkName);
    console.log(`Network: ${config.name} (Chain ID: ${config.chainId})\n`);
    
    // Get contract address from deployment or environment
    let contractAddress = process.env.FLASHLOAN_CONTRACT_ADDRESS;
    
    if (!contractAddress) {
        // Try to load from deployments
        const deploymentFile = path.join(__dirname, "..", "deployments", `${networkName}.json`);
        if (fs.existsSync(deploymentFile)) {
            const deployment = JSON.parse(fs.readFileSync(deploymentFile, "utf8"));
            contractAddress = deployment.contractAddress;
            console.log(`Loaded contract from deployment: ${contractAddress}\n`);
        } else {
            throw new Error(
                `Contract address not found. Set FLASHLOAN_CONTRACT_ADDRESS env var or deploy first.\n` +
                `Deploy with: npx hardhat run scripts/deployProduction.js --network ${networkName}`
            );
        }
    }
    
    // Get parameters
    const victimAddress = process.env.VICTIM_ADDRESS;
    if (!victimAddress) {
        throw new Error("VICTIM_ADDRESS environment variable is required");
    }
    
    const debtToken = process.env.DEBT_TOKEN || config.tokens.USDC;
    const collateralToken = process.env.COLLATERAL_TOKEN || config.tokens.WETH;
    const debtAmount = process.env.DEBT_AMOUNT || "5000000000"; // Default: 5000 USDC (6 decimals)
    
    console.log("=== Liquidation Parameters ===");
    console.log("Contract:", contractAddress);
    console.log("Victim:", victimAddress);
    console.log("Debt Token:", debtToken);
    console.log("Collateral Token:", collateralToken);
    console.log("Debt Amount:", debtAmount, "\n");
    
    // Get signer
    const [signer] = await ethers.getSigners();
    console.log("Signer:", signer.address);
    const balance = await ethers.provider.getBalance(signer.address);
    console.log("Balance:", ethers.formatEther(balance), "ETH\n");
    
    // Load contract
    const FlashLoanLiquidation = await ethers.getContractFactory("FlashLoanLiquidation");
    const contract = await FlashLoanLiquidation.attach(contractAddress);
    
    // Verify contract owner
    const owner = await contract.owner();
    if (owner.toLowerCase() !== signer.address.toLowerCase()) {
        console.log("⚠️  WARNING: You are not the contract owner!");
        console.log("   Owner:", owner);
        console.log("   You:", signer.address);
        console.log("   Proceeding anyway (anyone can call liquidation functions)...\n");
    }
    
    // Check victim health factor
    console.log("=== Pre-Liquidation Checks ===");
    const poolABI = [
        "function getUserAccountData(address user) view returns (uint256 totalCollateralBase, uint256 totalDebtBase, uint256 availableBorrowsBase, uint256 currentLiquidationThreshold, uint256 ltv, uint256 healthFactor)"
    ];
    const pool = await ethers.getContractAt(poolABI, config.aavePool);
    
    try {
        const userData = await pool.getUserAccountData(victimAddress);
        const healthFactor = userData.healthFactor;
        const hfFloat = Number(ethers.formatUnits(healthFactor, 27));
        
        console.log("Health Factor:", hfFloat.toFixed(4));
        console.log("Collateral:", ethers.formatUnits(userData.totalCollateralBase, 8));
        console.log("Debt:", ethers.formatUnits(userData.totalDebtBase, 8));
        
        if (hfFloat >= 1.0) {
            console.log("\n⚠️  WARNING: Health factor is above 1.0!");
            console.log("   Position is not liquidatable.");
            console.log("   Aborting liquidation.\n");
            return;
        }
        
        console.log("✓ Position is liquidatable!\n");
    } catch (error) {
        console.log("⚠️  Could not fetch user data:", error.message);
        console.log("   Proceeding anyway...\n");
    }
    
    // Check contract balance (for edge cases)
    const debtTokenContract = await ethers.getContractAt("IERC20", debtToken);
    const contractBalance = await debtTokenContract.balanceOf(contractAddress);
    console.log("Contract balance:", ethers.formatUnits(contractBalance, 6), "tokens");
    console.log("(Not required for flash loans, but helpful for edge cases)\n");
    
    // Execute liquidation
    console.log("=== Executing Liquidation ===");
    console.log("Sending transaction...");
    
    const debtAmountBigInt = BigInt(debtAmount);
    
    try {
        const tx = await contract.requestLiquidationLoanSimple(
            debtToken,
            debtAmountBigInt,
            victimAddress,
            collateralToken,
            {
                gasLimit: 1000000, // Adjust if needed
            }
        );
        
        console.log("Transaction hash:", tx.hash);
        console.log(`Explorer: ${config.explorer}/tx/${tx.hash}`);
        console.log("Waiting for confirmation...\n");
        
        const receipt = await tx.wait();
        console.log("✓ Transaction confirmed!");
        console.log("Block:", receipt.blockNumber);
        console.log("Gas used:", receipt.gasUsed.toString());
        console.log(`Explorer: ${config.explorer}/tx/${tx.hash}\n`);
        
        // Check results
        console.log("=== Post-Liquidation Check ===");
        const collateralAToken = await pool.getReserveData(collateralToken);
        const aTokenAddress = collateralAToken.aTokenAddress;
        const aToken = await ethers.getContractAt("IERC20", aTokenAddress);
        const profit = await aToken.balanceOf(contractAddress);
        
        if (profit > 0) {
            console.log("✓ Profit captured!");
            console.log("aToken balance:", ethers.formatEther(profit));
            console.log("\nTo withdraw profits, run:");
            console.log(`npx hardhat run scripts/withdrawProfit.js --network ${networkName}`);
        } else {
            console.log("⚠️  No profit detected in contract");
            console.log("   This could mean:");
            console.log("   - Liquidation was not profitable");
            console.log("   - Position was already liquidated");
            console.log("   - Swap consumed all collateral");
        }
        
    } catch (error) {
        console.error("\n❌ Transaction failed:", error.message);
        
        if (error.reason) {
            console.error("Reason:", error.reason);
        }
        
        if (error.message.includes("insufficient balance")) {
            console.error("\n💡 Tip: Flash loans don't require pre-funding, but check:");
            console.error("   - Victim position is still liquidatable");
            console.error("   - Debt amount is correct");
            console.error("   - Token addresses are correct");
        }
        
        if (error.message.includes("slippage")) {
            console.error("\n💡 Tip: Slippage protection triggered. Try:");
            console.error("   - Increasing maxSlippageBps in contract");
            console.error("   - Using a different swap path");
        }
        
        throw error;
    }
    
    console.log("\n=== Liquidation Complete ===");
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    });
