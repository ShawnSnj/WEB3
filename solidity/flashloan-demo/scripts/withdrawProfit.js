const { ethers } = require("hardhat");

/**
 * Withdraw Profit from Flash Loan Contract
 * 
 * This script withdraws profits (aTokens or tokens) from the flash loan contract.
 */

async function main() {
    console.log("=== Withdraw Profit from Flash Loan Contract ===\n");

    const FLASHLOAN_ADDRESS = process.env.FLASHLOAN_ADDRESS || "PASTE_YOUR_DEPLOYED_ADDRESS_HERE";
    const TOKEN_ADDRESS = process.env.TOKEN_ADDRESS || "0x4d5F47FA6A74757f35C14fD3a6Ef8E3C9BC514E8"; // Default: aWETH

    if (FLASHLOAN_ADDRESS === "PASTE_YOUR_DEPLOYED_ADDRESS_HERE") {
        console.error("❌ Error: Please set FLASHLOAN_ADDRESS environment variable");
        process.exit(1);
    }

    const [signer] = await ethers.getSigners();
    console.log("Signer:", signer.address);
    console.log("FlashLoan Contract:", FLASHLOAN_ADDRESS);
    console.log("Token to withdraw:", TOKEN_ADDRESS, "\n");

    // Get contract instances
    const flashLoanContract = await ethers.getContractAt("FlashLoanExample", FLASHLOAN_ADDRESS);
    const token = await ethers.getContractAt("IERC20", TOKEN_ADDRESS);

    // Check balance
    const balance = await token.balanceOf(FLASHLOAN_ADDRESS);
    console.log("Contract balance:", ethers.formatEther(balance), "tokens");

    if (balance === 0n) {
        console.log("⚠️  No balance to withdraw");
        process.exit(0);
    }

    // Check if it's an aToken (has UNDERLYING_ASSET_ADDRESS function)
    let isAToken = false;
    try {
        const aToken = await ethers.getContractAt("IAToken", TOKEN_ADDRESS);
        const underlying = await aToken.UNDERLYING_ASSET_ADDRESS();
        isAToken = true;
        console.log("Detected aToken - underlying asset:", underlying);
        console.log("\nWithdrawing aToken (will convert to underlying asset)...");

        const tx = await flashLoanContract.withdrawAToken(TOKEN_ADDRESS);
        console.log("Transaction sent:", tx.hash);
        await tx.wait();
        console.log("✓ Withdrawal complete!");

        // Check underlying balance
        const underlyingToken = await ethers.getContractAt("IERC20", underlying);
        const underlyingBalance = await underlyingToken.balanceOf(signer.address);
        console.log("Underlying balance received:", ethers.formatEther(underlyingBalance), "tokens");

    } catch (error) {
        // Not an aToken, use regular withdraw
        console.log("Withdrawing regular token...");
        const tx = await flashLoanContract.withdraw(TOKEN_ADDRESS);
        console.log("Transaction sent:", tx.hash);
        await tx.wait();
        console.log("✓ Withdrawal complete!");

        const finalBalance = await token.balanceOf(signer.address);
        console.log("Your balance:", ethers.formatEther(finalBalance), "tokens");
    }

    console.log("\n=== Complete ===");
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    });