const { ethers } = require("hardhat");

/**
 * Simple Flash Loan Example
 * 
 * This script demonstrates a basic flash loan execution.
 * Make sure to update FLASHLOAN_ADDRESS with your deployed contract address.
 */

async function main() {
    console.log("=== Simple Flash Loan Execution ===\n");

    // Configuration
    const FLASHLOAN_ADDRESS = process.env.FLASHLOAN_ADDRESS || "PASTE_YOUR_DEPLOYED_ADDRESS_HERE";
    const USDC_ADDRESS = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"; // Ethereum Mainnet
    const VICTIM_ADDRESS = process.env.VICTIM_ADDRESS || "0x0000000000000000000000000000000000000000";

    if (FLASHLOAN_ADDRESS === "PASTE_YOUR_DEPLOYED_ADDRESS_HERE") {
        console.error("❌ Error: Please set FLASHLOAN_ADDRESS environment variable or update the script");
        console.error("   Usage: FLASHLOAN_ADDRESS=0x... VICTIM_ADDRESS=0x... npx hardhat run scripts/simpleFlashLoan.js");
        process.exit(1);
    }

    if (VICTIM_ADDRESS === "0x0000000000000000000000000000000000000000") {
        console.error("❌ Error: Please set VICTIM_ADDRESS environment variable");
        console.error("   Usage: FLASHLOAN_ADDRESS=0x... VICTIM_ADDRESS=0x... npx hardhat run scripts/simpleFlashLoan.js");
        process.exit(1);
    }

    const [signer] = await ethers.getSigners();
    console.log("Signer:", signer.address);
    console.log("FlashLoan Contract:", FLASHLOAN_ADDRESS);
    console.log("Victim Address:", VICTIM_ADDRESS);
    console.log("Debt Token (USDC):", USDC_ADDRESS, "\n");

    // Get contract instance
    const flashLoanContract = await ethers.getContractAt("FlashLoanExample", FLASHLOAN_ADDRESS);

    // Check contract USDC balance
    const usdc = await ethers.getContractAt("IERC20", USDC_ADDRESS);
    const contractBalance = await usdc.balanceOf(FLASHLOAN_ADDRESS);
    console.log("Contract USDC Balance:", ethers.formatUnits(contractBalance, 6), "USDC");

    if (contractBalance < ethers.parseUnits("100", 6)) {
        console.log("⚠️  Warning: Contract has low USDC balance.");
        console.log("   Flash loan premium needs to be paid from contract balance.");
        console.log("   Recommended: At least 100 USDC for testing\n");
    } else {
        console.log("✓ Contract has sufficient balance\n");
    }

    // Amount to borrow (in USDC, 6 decimals)
    const debtAmount = ethers.parseUnits("10000", 6); // 10,000 USDC
    console.log("Executing flash loan liquidation...");
    console.log("Debt to cover:", ethers.formatUnits(debtAmount, 6), "USDC\n");

    try {
        const tx = await flashLoanContract.requestLiquidationLoanWithWETH(
            USDC_ADDRESS,
            debtAmount,
            VICTIM_ADDRESS
        );

        console.log("Transaction sent:", tx.hash);
        console.log("Waiting for confirmation...");

        const receipt = await tx.wait();
        console.log("✓ Transaction confirmed!");
        console.log("  Block:", receipt.blockNumber);
        console.log("  Gas used:", receipt.gasUsed.toString());

        // Check for events
        console.log("\nChecking contract balances after liquidation...");
        const aWETH_ADDRESS = "0x4d5F47FA6A74757f35C14fD3a6Ef8E3C9BC514E8"; // Mainnet aWETH
        const aWETH = await ethers.getContractAt("IERC20", aWETH_ADDRESS);
        const aWETHBalance = await aWETH.balanceOf(FLASHLOAN_ADDRESS);
        
        console.log("aWETH Balance:", ethers.formatEther(aWETHBalance), "aWETH");
        
        if (aWETHBalance > 0) {
            console.log("✓ Liquidation successful! Profit captured as aWETH.");
            console.log("\nTo withdraw profits, run:");
            console.log(`  npx hardhat run scripts/withdrawProfit.js --network mainnet`);
        }

    } catch (error) {
        console.error("❌ Transaction failed:", error.message);
        
        if (error.message.includes("insufficient balance")) {
            console.error("\n💡 Solution: Fund the contract with USDC to cover the flash loan premium");
        } else if (error.message.includes("health factor")) {
            console.error("\n💡 Solution: The victim position may not be liquidatable (HF >= 1.0)");
        }
        
        process.exit(1);
    }

    console.log("\n=== Complete ===");
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    });